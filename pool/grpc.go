package pool

import (
	"context"
	"errors"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/log"
	"github.com/582033/gin-utils/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrClosed  = errors.New("grpc pool: client pool is closed")
	ErrTimeout = errors.New("grpc pool: client pool timed out")
)

type conf struct {
	Addr        string   `json:"addr"`
	Size        int      `json:"size"`
	MaxLife     int      `json:"max_life"`
	MaxUseCount uint64   `json:"max_use_count"`
	Ctx         []string `json:"ctx"`
}

type Factory func() (*grpc.ClientConn, error)

type FactoryWithContext func(context.Context) (*grpc.ClientConn, error)

type Pool struct {
	clients         *Queue //free lock queue
	factory         FactoryWithContext
	maxLifeDuration time.Duration //最大生命周期
	maxUseCount     uint64        //最多使用次数
	close           int32
	unHealthyCount  uint64 //当前连接池中 不健康的client数量
	cap             int
}

type ClientConn struct {
	*grpc.ClientConn
	pool          *Pool
	timeInitiated time.Time
	use           uint64 //当前连接被使用数量
	useCount      uint64
	unhealthy     bool
}

var client = sync.Map{}

var once = sync.Once{}

func GRPC(ctx context.Context, name string) *ClientConn {
	once.Do(func() {
		initClient()
	})
	if v, ok := client.Load(name); ok {
		conn, _ := v.(*Pool).Get(ctx)
		return conn
	}
	return nil
}

func clientFactory(addr string, key []string) (*grpc.ClientConn, error) {
	opts := []retry.CallOption{
		retry.WithBackoff(retry.BackoffLinear(10 * time.Millisecond)),
		retry.WithCodes(codes.NotFound, codes.Aborted),
		retry.WithMax(5),
	}

	conn, err := grpc.Dial(addr,
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1024*1024*8), //8M
			grpc.MaxCallSendMsgSize(1024*1024*8),
		),
		grpc.WithStreamInterceptor(retry.StreamClientInterceptor(opts...)),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(
			retry.UnaryClientInterceptor(opts...),
			middleware.ClientCtxInterceptor(key...),
		),
		),
	)

	if err != nil {
		log.Error("grpc not connect:" + err.Error())
		return nil, err
	}
	return conn, nil
}

func initClient() {
	var data map[string]conf
	if err := config.Get("grpc").Scan(&data); err != nil {
		log.Fatal("error parsing grpc configuration file ", err)
		return
	}
	for key, c := range data {
		log.Debugf("grpc config %s: %+v", key, c)
		err := Register(key, c.Addr, c.Size, time.Duration(c.MaxLife)*time.Second, c.MaxUseCount, c.Ctx)
		if err != nil {
			log.Error("grpc error ", err)
			continue
		}
	}
}

func Register(name string, addr string, capacity int, maxLifeDuration time.Duration, maxUseCount uint64, key []string) error {
	if maxLifeDuration == 0 {
		maxLifeDuration = 3600 * time.Second
	}
	if maxUseCount == 0 {
		maxUseCount = 256
	}
	pool, err := New(func() (*grpc.ClientConn, error) {
		log.Debugf("connect grpc %s: %s", name, addr)
		return clientFactory(addr, key)
	}, capacity, maxLifeDuration, maxUseCount)
	if err != nil {
		log.Error(err)
		return err
	}
	client.Store(name, pool)
	return nil
}

func New(factory Factory, capacity int, maxLifeDuration time.Duration, maxUseCount uint64) (*Pool, error) {
	return NewWithContext(context.Background(), func(ctx context.Context) (*grpc.ClientConn, error) { return factory() },
		capacity, maxLifeDuration, maxUseCount)
}

func NewWithContext(ctx context.Context, factory FactoryWithContext, capacity int, maxLifeDuration time.Duration, maxUseCount uint64) (*Pool, error) {

	if capacity < 2 {
		capacity = 2 //default cap 2
	}

	p := &Pool{
		clients:         NewQueue(),
		factory:         factory,
		maxLifeDuration: maxLifeDuration,
		maxUseCount:     maxUseCount,
		cap:             capacity,
		close:           0,
		unHealthyCount:  0,
	}

	for i := 0; i < capacity; i++ {
		c, err := factory(ctx)
		if err != nil {
			return nil, err
		}
		p.clients.Enqueue(newClientConn(p, c))
	}
	go p.healthy(ctx)
	return p, nil
}

func newClientConn(p *Pool, c *grpc.ClientConn) *ClientConn {
	return &ClientConn{
		ClientConn:    c,
		pool:          p,
		timeInitiated: time.Now(),
		unhealthy:     false,
		use:           0,
		useCount:      0,
	}
}

func (p *Pool) IsClosed() bool {
	return atomic.LoadInt32(&(p.close)) == 1
}

//Close pool
func (p *Pool) Close() {
	if atomic.CompareAndSwapInt32(&(p.close), 0, 1) {
		//遍历关闭队列中的conn
		for v := p.clients.Dequeue(); v != nil; {
			_ = v.(*ClientConn).ClientConn.Close()
		}
	}
}

func (p *Pool) Get(ctx context.Context) (*ClientConn, error) {
	if p.IsClosed() {
		return nil, ErrClosed
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ErrTimeout
		default:
			//从队列头取出
			if v := p.clients.Dequeue(); v != nil {
				conn := v.(*ClientConn)
				p.clients.Enqueue(conn) //加到队尾
				if conn.IsHealthy() {
					atomic.AddUint64(&(conn.use), 1)
					atomic.AddUint64(&(conn.useCount), 1)
					return conn, nil
				}
			}
		}
	}
}

func (c *ClientConn) Close() {
	atomic.AddUint64(&(c.use), ^uint64(0))
	maxDuration := c.pool.maxLifeDuration
	if maxDuration > 0 && c.timeInitiated.Add(maxDuration).Before(time.Now()) {
		c.Unhealthy()
	}
	maxUseCount := c.pool.maxUseCount
	if maxUseCount > 0 && c.useCount > maxUseCount {
		c.Unhealthy()
	}
}

func (p *Pool) healthy(ctx context.Context) {
	for !p.IsClosed() {
		if v := p.clients.Dequeue(); v != nil {
			conn := v.(*ClientConn)
			if !conn.IsHealthy() && conn.use == 0 {
				c, err := p.factory(ctx)
				if err != nil {
					log.Error(err)
				} else {
					_ = conn.ClientConn.Close()
					atomic.AddUint64(&(p.unHealthyCount), ^uint64(0))
					conn = newClientConn(p, c)
				}
			}
			p.clients.Enqueue(conn)
		}
		time.Sleep(500 * time.Millisecond) //500ms
	}
}

func (p *Pool) Capacity() int {
	if p.IsClosed() {
		return 0
	}
	return p.cap
}

func (c *ClientConn) Unhealthy() {
	if c.pool.unHealthyCount < uint64(c.pool.cap/2) {
		c.unhealthy = true
		atomic.AddUint64(&(c.pool.unHealthyCount), 1)
		log.Debugf("set unhealthy success: %+v", c)
	}
}

func (c *ClientConn) IsHealthy() bool {
	return c.unhealthy == false
}

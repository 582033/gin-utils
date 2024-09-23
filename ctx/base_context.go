package ctx

import (
	"context"
	"github.com/google/uuid"
	"sync"
	"time"
)

const (
	BaseContextRequestIDKey = "_base_ctx_key_request_id"
	BaseContextOperatorKey  = "_base_ctx_key_operator"
	BaseContextSourceKey    = "_base_ctx_key_source"
)

type Base struct {
	keysMutex *sync.RWMutex
	keys      map[string]interface{}
}

//base context 接口
type BaseContext interface {
	context.Context
	GetRequestID() string
	GetSource() string
	GetOperator() string
	Get(key string) (interface{}, bool)
	Set(key string, val interface{})
}

func NewNilBaseContext() *Base {
	return &Base{
		keysMutex: &sync.RWMutex{},
		keys:      map[string]interface{}{},
	}
}

func (c *Base) SetRequestId(v string) {
	c.set(BaseContextRequestIDKey, v)
}

func (c *Base) SetSource(v string) {
	c.set(BaseContextSourceKey, v)
}

func (c *Base) SetOperator(v string) {
	c.set(BaseContextOperatorKey, v)
}

func (c *Base) GetRequestID() string {
	if v, ok := c.get(BaseContextRequestIDKey); ok && v != nil && v.(string) != "" {
		return v.(string)
	}
	uuid := uuid.New().String()
	c.set(BaseContextRequestIDKey, uuid)
	return uuid
}

func (c *Base) GetSource() string {
	if v, ok := c.get(BaseContextSourceKey); ok && v != nil && v.(string) != "" {
		return v.(string)
	}
	return ""
}

func (c *Base) GetOperator() string {
	if v, ok := c.get(BaseContextOperatorKey); ok && v != nil && v.(string) != "" {
		return v.(string)
	}
	return ""
}

func (c *Base) set(key string, value interface{}) {
	if c.keysMutex == nil {
		c.keysMutex = &sync.RWMutex{}
	}

	c.keysMutex.Lock()
	if c.keys == nil {
		c.keys = make(map[string]interface{})
	}

	c.keys[key] = value
	c.keysMutex.Unlock()
}

func (c *Base) Get(key string) (value interface{}, exists bool) {
	return c.get(key)
}

func (c *Base) Set(key string, val interface{}) {
	c.set(key, val)
}

func (c *Base) get(key string) (value interface{}, exists bool) {
	if c.keysMutex == nil {
		c.keysMutex = &sync.RWMutex{}
	}

	c.keysMutex.RLock()
	value, exists = c.keys[key]
	c.keysMutex.RUnlock()
	return
}

func (c *Base) Deadline() (deadline time.Time, ok bool) {
	return
}

func (c *Base) Done() <-chan struct{} {
	return nil
}

func (c *Base) Err() error {
	return nil
}

func (c *Base) Value(key interface{}) interface{} {
	if key == 0 {
		return nil
	}
	if keyAsString, ok := key.(string); ok {
		val, _ := c.get(keyAsString)
		return val
	}
	return nil
}

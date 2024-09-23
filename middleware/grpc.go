package middleware

import (
	"context"
	"encoding/json"
	"github.com/582033/gin-utils/apm"
	ctx2 "github.com/582033/gin-utils/ctx"
	"github.com/582033/gin-utils/log"
	"github.com/582033/gin-utils/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"time"
)

func UnaryDebugInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		reqBytes, _ := json.Marshal(req)
		log.Debugf("[%s] grpc request method:[%s], body: %s", ctx.Value(ctx2.BaseContextRequestIDKey), info.FullMethod,
			util.Bytes2str(reqBytes))
		s := time.Now()
		counter := apm.Counter(info.FullMethod, "current") //当前请求量
		//总请求计数
		if meter := apm.Meter(info.FullMethod, "total"); meter != nil {
			meter.Mark(1)
		}
		if counter != nil {
			counter.Inc(1)
		}
		res, err := handler(ctx, req) //grpc服务处理逻辑
		if counter != nil {
			counter.Dec(1)
		}

		if meter := apm.Meter(info.FullMethod, "error"); meter != nil && err != nil {
			meter.Mark(1)
		}
		apm.Histograms(info.FullMethod, "execTime").Update(time.Since(s).Milliseconds())
		resBytes, _ := json.Marshal(res)
		log.Debugf("[%s] grpc response method:[%s], body: %s, cost: %vms", ctx.Value(ctx2.BaseContextRequestIDKey),
			info.FullMethod, util.Bytes2str(resBytes), time.Since(s).Milliseconds())
		return res, err
	}
}

func ClientCtxInterceptor(key ...string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string, req, resp interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) (err error) {
		reqBytes, _ := json.Marshal(req)
		log.Debugf("[%s] grpc request method:[%s], body: %s", ctx.Value(ctx2.BaseContextRequestIDKey), method,
			util.Bytes2str(reqBytes))
		s := time.Now()
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.Pairs()
		}
		for _, v := range key {
			value := ctx.Value(v)
			if val, ok := value.(string); ok && val != "" {
				md[v] = []string{val}
			}
		}
		err = invoker(metadata.NewOutgoingContext(ctx, md), method, req, resp, cc, opts...)
		resBytes, _ := json.Marshal(resp)
		log.Debugf("[%s] grpc response method:[%s], body: %s, cost: %vms", ctx.Value(ctx2.BaseContextRequestIDKey),
			method, util.Bytes2str(resBytes), time.Since(s).Milliseconds())
		return err
	}
}

func ServerCtxInterceptor(key ...string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.Pairs()
		}

		for _, v := range key {
			vals := md[v]
			if len(vals) >= 1 {
				ctx = context.WithValue(ctx, v, vals[0])
			}
		}
		//requestIDs := md[ctx2.BaseContextRequestIDKey]
		//if len(requestIDs) >= 1 {
		//	ctx = context.WithValue(ctx, ctx2.BaseContextRequestIDKey, requestIDs[0])
		//	return handler(ctx, req)
		//}
		//
		//// Generate request ID and set context if not exists.
		//requestID := uuid.New().String()
		//ctx = context.WithValue(ctx, ctx2.BaseContextRequestIDKey, requestID)
		return handler(ctx, req)
	}
}

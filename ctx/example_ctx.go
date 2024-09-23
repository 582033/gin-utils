package ctx

import (
	"context"
	"github.com/openzipkin/zipkin-go/propagation/b3"
	"google.golang.org/grpc/metadata"
)

type ExampleContext interface {
	BaseContext
	TestFunc() string
}

//自定义Context
type EContext struct {
	Base
}

func NewExampleContext(c context.Context) *EContext {
	ctx := &EContext{
		Base: *NewNilBaseContext(),
	}
	//解析grpc metadata
	md, _ := metadata.FromIncomingContext(c)

	//解析调用链sn
	if ext, err := b3.ExtractGRPC(&md)(); err == nil {
		ctx.SetRequestId(ext.TraceID.String())
	}

	return ctx
}

func NewNilContext() *EContext {
	return &EContext{
		Base: *NewNilBaseContext(),
	}
}

func (ctx *EContext) TestFunc() string {
	return "testFunc"
}

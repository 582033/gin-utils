package grpcserver

import (
	"fmt"
	"github.com/582033/gin-utils/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
)

type app struct {
	port           int
	registerServer func() *grpc.Server
}

func NewApp(port int) *app {
	return &app{
		port: port,
	}
}

//RegisterServer 注册 rpc server
func (a *app) RegisterServer(f func() *grpc.Server) *app {
	a.registerServer = f
	return a
}

//Start 启动grpc server
func (a *app) Start() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := a.registerServer()
	reflection.Register(s)
	log.Info("grpc server start and listening on:", a.port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

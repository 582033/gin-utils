// +build windows

package server

import (
	"github.com/582033/gin-utils/log"
	"net/http"
)

//Run 运行
func (v *App) Run(addr string) {
	log.Info("server start and listening on:", addr)
	srv := &http.Server{Addr: addr, Handler: v.Engine}
	err := srv.ListenAndServe()

	if err != nil {
		log.Error(err)
	}
}

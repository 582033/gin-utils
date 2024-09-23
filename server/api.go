package server

import (
	"github.com/gin-gonic/gin"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/middleware"
	"net/http"
	"os"
	"time"
)

var startTime = time.Now()

var defaultSkipPaths = []string{"/favicon.ico", "/"}

//App 定义一个应用
type App struct {
	Engine *gin.Engine
}

//RegisterAPIRouter 注册路由规则
func (v *App) RegisterAPIRouter(fun func(router *gin.Engine)) *App {
	fun(v.Engine)
	return v
}

// NewApp ..
func NewApp(middlewareHandler ...gin.HandlerFunc) *App {
	return baseNewApp(defaultSkipPaths, middlewareHandler...)
}

// NewAppWithSkip ..
func NewAppWithSkip(skipPaths []string, middlewareHandler ...gin.HandlerFunc) *App {
	return baseNewApp(skipPaths, middlewareHandler...)
}

// baseNewApp
func baseNewApp(skipPaths []string, middlewareHandler ...gin.HandlerFunc) *App {
	router := gin.New()
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: skipPaths,
	}), middleware.Gin(), middleware.Debug(skipPaths))
	router.Use(middlewareHandler...)
	router.Any("/sys/health", health)
	router.Any("/", health)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code": http.StatusNotFound,
		})
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code": http.StatusNotFound,
		})
	})
	return &App{
		Engine: router,
	}
}


//health 健康检查
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"pid":        os.Getpid(),
		"start_time": startTime,
		"code":       http.StatusOK,
		"name":       config.Get("service.name").String(""),
	})
}

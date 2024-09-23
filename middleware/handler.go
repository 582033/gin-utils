package middleware

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/582033/gin-utils/apm"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/ctx"
	"github.com/582033/gin-utils/log"
	"github.com/582033/gin-utils/util"
	"io/ioutil"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

func Gin() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if p := recover(); p != nil {
				log.WithCtx(c).Errorf("panic: %v, detail: %s", p, string(debug.Stack()))
				c.JSON(http.StatusInternalServerError, fmt.Sprintf("%s, %s", http.StatusText(http.StatusInternalServerError), p))
				c.Abort()
				return
			}
		}()
		c.Next()
	}
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

//Debug debug mode
func Debug(skipPaths []string) gin.HandlerFunc {
	return func(c *gin.Context) {

		if util.InSliceStr(c.Request.URL.Path, skipPaths) != -1 {
			c.Next()
			return
		}

		// 2020/5/28 17:44 kangyunjie 上传文件时，未打印日志
		requestId := getRequestID(c, "X-Request-ID")
		c.Set(ctx.BaseContextRequestIDKey, requestId)
		if !strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data;") {
			var bodyBytes []byte
			if c.Request.Body != nil {
				bodyBytes, _ = ioutil.ReadAll(c.Request.Body)
			}
			log.Debugf("[%s] api request, clientIp: %s, method: %s, url: %s, headers: %s, query params: %s, params: %s, body: %s", requestId, c.ClientIP(),
				c.Request.Method, c.Request.URL.Path, c.Request.Header, c.Request.URL.RawQuery, c.Request.Form, subString(string(bodyBytes)))
			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		if fullPath := c.FullPath(); fullPath != "" {
			s := time.Now()
			counter := apm.Counter(fullPath, "current") //当前请求量
			//总请求计数
			if meter := apm.Meter(fullPath, "total"); meter != nil {
				meter.Mark(1)
			}
			if counter != nil {
				counter.Inc(1)
			}
			blw := &responseBodyWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
			c.Writer = blw
			c.Next()
			// 2022/4/12 17:49 kangyj 过滤下载接口
			var respBody string
			if !strings.Contains(c.Writer.Header().Get("Content-Type"), "application/octet-stream") {
				respBody = subString(blw.body.String())
			}
			since := time.Since(s)
			log.Debugf("[%s] api response, url: %s, body: %v, cost: %vms", requestId, c.Request.URL.Path, respBody, since.Milliseconds())
			expectedTimeout := config.Get("service.debug.timeout").Float64(3)
			if since.Seconds() > expectedTimeout {
				log.Warnf("[%s] api timeout, url: %s, expected: %v, actual: %v", requestId, c.Request.URL.Path, expectedTimeout, since.Seconds())
			}

			if counter != nil {
				counter.Dec(1)
			}
			if status := apm.Meter(fullPath, strconv.Itoa(c.Writer.Status())); status != nil {
				status.Mark(1)
			}
			apm.Histograms(fullPath, "execTime").Update(time.Since(s).Milliseconds())
		}
	}
}

func subString(body string) string {
	maxPrint := config.Get("service.debug.max_print").Int(500)
	bodyRunes := []rune(body)
	if len(bodyRunes) > maxPrint {
		return string(bodyRunes[:maxPrint]) + "..."
	}
	return body
}

func getSource(c *gin.Context) string {
	source := c.GetHeader("source")
	if source != "" {
		return source
	}
	source = c.Query("source")
	if source != "" {
		return source
	}
	source = c.PostForm("source")
	if source != "" {
		return source
	}
	return source
}

func getOperator(c *gin.Context) string {
	operator := c.GetHeader("operator")
	if operator != "" {
		return operator
	}
	operator = c.Query("operator")
	if operator != "" {
		return operator
	}
	operator = c.PostForm("operator")
	if operator != "" {
		return operator
	}
	return operator
}

func getRequestID(c *gin.Context, requestIDKey string) string {
	requestId := c.GetHeader(requestIDKey)
	if requestId != "" {
		return requestId
	}
	requestId = c.Query(requestIDKey)
	if requestId != "" {
		return requestId
	}
	requestId = c.PostForm(requestIDKey)
	if requestId != "" {
		return requestId
	}
	return uuid.New().String()
}

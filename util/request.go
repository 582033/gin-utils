package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/582033/gin-utils/ctx"
	"github.com/582033/gin-utils/log"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/opkit/gorequest"
)

type request struct {
	timeOut           time.Duration
	retryNum          int
	retryTime         time.Duration
	checkHTTPStatus   bool
	bounceToRawString bool
	log               bool
}

type RequestOption func(opt *request)

func NewHttpRequestV2(opts ...RequestOption) *request {
	r := &request{ //default value
		timeOut:           time.Duration(3000) * time.Millisecond,
		retryNum:          2,
		retryTime:         time.Duration(1000) * time.Millisecond,
		checkHTTPStatus:   true,
		log:               true,
		bounceToRawString: false,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func RequestOptionTimeOut(millisecond int) RequestOption {
	return func(opt *request) {
		opt.timeOut = time.Duration(millisecond) * time.Millisecond
	}
}

func RequestOptionRetryNum(num int) RequestOption {
	return func(opt *request) {
		opt.retryNum = num
	}
}

func RequestOptionRetryTime(millisecond int) RequestOption {
	return func(opt *request) {
		opt.retryTime = time.Duration(millisecond) * time.Millisecond
	}
}

func RequestOptionDisableLog() RequestOption {
	return func(opt *request) {
		opt.log = false
	}
}

func RequestOptionDisableCheckHTTPStatus() RequestOption {
	return func(opt *request) {
		opt.checkHTTPStatus = false
	}
}

func RequestOptionEnableBounceToRawString() RequestOption {
	return func(opt *request) {
		opt.bounceToRawString = true
	}
}

// Deprecated: Use NewHttpRequestV2
func NewHttpRequest(timeOut int, retryNum int, retryTime int) *request {
	return &request{
		timeOut:         time.Duration(timeOut) * time.Millisecond,
		retryNum:        retryNum,
		retryTime:       time.Duration(retryTime) * time.Millisecond,
		checkHTTPStatus: true,
		log:             true,
	}
}

func (r *request) BounceToRawString(v bool) *request {
	r.bounceToRawString = v
	return r
}

func (r *request) Log(v bool) *request {
	r.log = v
	return r
}

// Deprecated
func (r *request) Request(api string, method string, params interface{}, header map[string]string) ([]byte, error) {
	return r.RequestV2(context.Background(), api, method, params, header)
}

// 发送http请求
// 重试三次间隔一分钟
// api地址 请求方法 请求参数 请求头设置 返回数据解析
func (r *request) RequestV2(c context.Context, api string, method string, params interface{}, header map[string]string) ([]byte, error) {
	if api == "" {
		return nil, errors.New("api url null")
	}
	//创建request请求对象
	//设置重试状态
	request := gorequest.New().CustomMethod(strings.ToUpper(method), api)
	if r.timeOut > 0 {
		request = request.Timeout(r.timeOut)
	}
	if r.retryNum > 0 {
		request = request.Retry(r.retryNum, r.retryTime, http.StatusBadRequest, http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout)
	}
	request.BounceToRawString = r.bounceToRawString
	//设置请求参数
	if params != nil {
		if strings.ToUpper(method) == http.MethodGet {
			request.Query(params)
		} else {
			request.Send(params)
		}
	}
	//设置请求头
	for hk, hv := range header {
		request.Set(hk, hv)
	}
	//发送请求
	ret, body, errs := request.EndBytes()

	curlCommand, err := request.AsCurlCommand()
	log.Debugf("[%v] %s %v", c.Value(ctx.BaseContextRequestIDKey), curlCommand, err)

	//验证错误
	if len(errs) > 0 {
		if r.log {
			log.Debugf("[%v] errs: %v+", c.Value(ctx.BaseContextRequestIDKey), errs)
		}
		return nil, errs[0]
	}
	if r.log {
		log.Debugf("[%v] status: %v header: %v body: %s", c.Value(ctx.BaseContextRequestIDKey), ret.Status, ret.Header, Bytes2str(body))
	}
	//验证返回状态
	if ret.StatusCode != http.StatusOK && r.checkHTTPStatus {
		return nil, fmt.Errorf("http code error %d", ret.StatusCode)
	}
	if len(body) == 0 {
		return nil, errors.New("body null")
	}
	return body, nil
}

// RequestBindJSON 发送http请求 返回结果解析到json中
func (r *request) BindJSON(api string, method string, params interface{}, header map[string]string, res interface{}) error {
	body, err := r.Request(api, method, params, header)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, &res)
}

//Download 下载文件
func (r *request) Download(fileURL string, method string, params interface{}, header map[string]string, dst string) error {
	if fileURL == "" {
		return errors.New("fileURL url null")
	}
	//创建request请求对象
	//设置重试状态
	request := gorequest.New().CustomMethod(strings.ToUpper(method), fileURL)
	if r.timeOut > 0 {
		request = request.Timeout(r.timeOut)
	}
	if r.retryNum > 0 {
		request = request.Retry(r.retryNum, r.retryTime, http.StatusBadRequest, http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout)
	}
	//设置请求参数
	if params != nil {
		if strings.ToUpper(method) == http.MethodGet {
			request.Query(params)
		} else {
			request.Send(params)
		}
	}
	//设置请求头
	for hk, hv := range header {
		request.Set(hk, hv)
	}
	//发送请求
	req, err := request.MakeRequest()
	if err != nil {
		log.Error(err)
		return err
	}

	ret, err := request.Client.Do(req)
	if err != nil {
		log.Error(err)
		return err
	}
	//验证返回状态
	if ret.StatusCode != http.StatusOK && r.checkHTTPStatus {
		return fmt.Errorf("http code error %d", ret.StatusCode)
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	defer ret.Body.Close()
	_, err = io.Copy(out, ret.Body)
	return err
}

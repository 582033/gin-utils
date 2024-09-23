package aliyun

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/google/uuid"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/ctx"
	"github.com/582033/gin-utils/log"
	"github.com/582033/gin-utils/mime"
)

var (
	once = sync.Once{}
	conn = make(map[string]*OSS)
)

type OSS struct {
	bucketClient *oss.Bucket
	config       OSSConfig
}

type OSSConfig struct {
	BucketName      string `json:"bucket_name"`
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	Prefix          string `json:"prefix"`
	Domain          string `json:"domain"`
}

type putResp struct {
	Bucket string `json:"bucket"`
	Domain string `json:"domain"`
	Key    string `json:"path"`
	Acl    string `json:"acl"`
}

func (resp *putResp) URL() string {
	return fmt.Sprintf("%s/%s", resp.Domain, resp.Key)
}

// Client ...
func (client *OSS) Client() (*oss.Bucket, error) {
	return client.bucketClient, nil
}

// PutFromFilePublic 上传到阿里云 返回访问地址
func (client *OSS) PutFromFilePublic(ctx ctx.BaseContext, src string) (*putResp, error) {
	return client.PutFromFile(ctx, src, genDst(src), oss.ACLPublicRead)
}

func (client *OSS) PutFromFilePrivate(ctx ctx.BaseContext, src string) (*putResp, error) {
	return client.PutFromFile(ctx, src, genDst(src), oss.ACLPrivate)
}

func (client *OSS) PutFromFile(ctx ctx.BaseContext, src, dst string, aclType oss.ACLType) (*putResp, error) {
	fd, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	resp, err := client.put(ctx, fd, dst, aclType)
	return resp, err
}

// PutFromFilePublic 上传到阿里云 返回访问地址
func (client *OSS) PutPublic(ctx ctx.BaseContext, file *multipart.FileHeader) (*putResp, error) {
	dst := genDst(file.Filename)
	fd, err := file.Open()
	if err != nil {
		return nil, err
	}
	resp, err := client.put(ctx, fd, dst, oss.ACLPublicRead)
	return resp, err
}

func (client *OSS) PutPrivate(ctx ctx.BaseContext, file *multipart.FileHeader) (*putResp, error) {
	dst := genDst(file.Filename)
	fd, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	resp, err := client.put(ctx, fd, dst, oss.ACLPrivate)
	return resp, err
}

func (client *OSS) GetObject(key string) (io.ReadCloser, error) {
	return client.bucketClient.GetObject(key)
}

type SignURL struct {
	ContentType string `json:"content_type,omitempty"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	Expire      int64  `json:"expire"`
}

// Deprecated: Use GetSignURLWithOptions
func (client *OSS) GetSignURL(key string, expiredInSec int64) (*SignURL, error) {
	return client.signURL(key, expiredInSec, true, http.MethodGet)
}

// Deprecated: Use GetSignURLWithOptions
func (client *OSS) GetSignURLWithMethod(key string, expiredInSec int64, method string) (*SignURL, error) {
	return client.signURL(key, expiredInSec, true, method)
}

func (client *OSS) GetSignURLWithOptions(key string, expiredInSec int64, method string, options ...oss.Option) (*SignURL, error) {
	return client.signURL(key, expiredInSec, true, method, options...)
}

func (client *OSS) GetSignURLNoExtWithOptions(key string, expiredInSec int64, method string, options ...oss.Option) (*SignURL, error) {
	return client.signURL(key, expiredInSec, false, method, options...)
}

func (client *OSS) signURL(key string, expiredInSec int64, checkExt bool, method string, options ...oss.Option) (*SignURL, error) {
	urlObj := &SignURL{
		Method: method,
		Expire: expiredInSec,
	}
	if checkExt {
		t := mime.TypeByExtension(filepath.Ext(key))
		options = append(options, oss.ContentType(t))
		urlObj.ContentType = t
	}
	url, err := client.bucketClient.SignURL(key, oss.HTTPMethod(method), expiredInSec, options...)
	if err != nil {
		return nil, err
	}

	urlObj.URL = url
	return urlObj, nil
}

func (client *OSS) DeleteObject(ctx ctx.BaseContext, key string) error {
	log.Infof("[%s] oss delete bucketName: %s, key: %s", ctx.GetRequestID(), client.config.BucketName, key)
	return client.bucketClient.DeleteObject(key)
}

func (client *OSS) put(ctx ctx.BaseContext, file io.Reader, dst string, acl oss.ACLType) (*putResp, error) {
	if dst == "" {
		return nil, errors.New("params error")
	}

	if client.config.Prefix != "" {
		dst = strings.Trim(client.config.Prefix, "/") + "/" + strings.TrimLeft(dst, "/")
	}

	options := []oss.Option{oss.ObjectACL(acl)}
	if err := client.bucketClient.PutObject(dst, file, options...); err != nil {
		return nil, err
	}
	resp := &putResp{
		Domain: strings.Trim(client.config.Domain, "/"),
		Key:    dst,
		Acl:    string(acl),
		Bucket: client.config.BucketName,
	}
	log.Infof("[%s] oss put resp: %+v", ctx.GetRequestID(), resp)
	return resp, nil
}

// Bucket 对外获取oss实例
func Bucket(key string) *OSS {
	once.Do(func() {
		initClient()
	})
	if db, ok := conn[key]; ok && db != nil {
		return db
	}
	return nil
}

func DefaultBucket() *OSS {
	return Bucket("default")
}

func NewOSS(conf OSSConfig) (*OSS, error) {
	client, err := oss.New(conf.Endpoint, conf.AccessKeyID, conf.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	bucket, err := client.Bucket(conf.BucketName)
	if err != nil {
		return nil, err
	}

	return &OSS{
		bucketClient: bucket,
		config:       conf,
	}, nil
}

func initClient() {
	var data map[string]OSSConfig
	if err := config.Get("oss").Scan(&data); err != nil {
		log.Fatal("error parsing oss configuration file ", err)
		return
	}
	for key, conf := range data {
		client, err := NewOSS(conf)
		if err != nil {
			log.Error("NewOSS error ", err)
			continue
		}
		conn[key] = client
	}
}

func genDst(src string) string {
	return fmt.Sprintf("%s/%s%s", time.Now().Format("2006-01-02"), uuid.New().String(), path.Ext(src))
}

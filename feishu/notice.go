package feishu

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/patrickmn/go-cache"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/worker"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

//https://open.feishu.cn/document/ukTMukTMukTM/uMDMxEjLzATMx4yMwETM

const AccessTokenApi = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal/"
const SearchUserIDApi = "https://open.feishu.cn/open-apis/user/v1/batch_get_id"
const ImgUploadApi = "https://open.feishu.cn/open-apis/image/v4/put/"

//默认最大支持1024个待发送任务
var jobLength = 1024
var jobQueue = make(chan worker.Job, jobLength)

//2个发送线程
var dispatcher = worker.NewDispatcher(2)

var feishuCache = cache.New(5*time.Minute, 10*time.Minute)

type Content interface {
	SetValue(val string, attr ...interface{}) Content
	IsValue() bool
}

type textContent struct {
	Tag  string `json:"tag"`
	Text string `json:"text"`
}

type atContent struct {
	Tag    string `json:"tag"`
	UserID string `json:"user_id"`
}
type imgContent struct {
	Tag      string `json:"tag"`
	ImageKey string `json:"image_key"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type msgBody struct {
	Title   string      `json:"title"`
	Content [][]Content `json:"content"`
}

type msgPostBody struct {
	MsgType string `json:"msg_type"`
	Content struct {
		Post map[string]msgBody `json:"post"`
	} `json:"content"`
}

type user struct {
	OpenID string `json:"open_id"`
	UserID string `json:"user_id"`
}

type emailUserList struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		EmailUsers map[string][]user `json:"email_users"`
	} `json:"data"`
}

type mobileUserList struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		MobileUsers map[string][]user `json:"mobile_users"`
	} `json:"data"`
}

type uploadImgResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ImageKey string `json:"image_key"`
	} `json:"data"`
}

func init() {
	dispatcher.Run(jobQueue)
}

func SendV2Adapter(title string, con ...Content) {
	sendV2("", title, con...)
}

func SendV3(webhook, title, text, at string) {
	//同步发送
	if config.Get("feishu.sync").Bool(true) {
		send(webhook, title, text, at)
		return
	}
	asyncSend(webhook, title, text, at)
}

func SendV2(title, text, at string) {
	//同步发送
	if config.Get("feishu.sync").Bool(true) {
		send("", title, text, at)
		return
	}
	asyncSend("", title, text, at)
}

func asyncSend(webhook, title string, text string, at string) {
	//当发送对列已满时，考虑到飞书发送限流等问题，新增发送任务直接丢弃
	if len(jobQueue) == jobLength {
		return
	}
	//异步发送
	job := worker.Job{
		Data: struct {
			Title string
			Text  string
			At    string
		}{Title: title, Text: text},
		Proc: func(i interface{}) {
			data := i.(struct {
				Title string
				Text  string
				At    string
			})
			send(webhook, data.Title, data.Text, at)
		},
	}

	select {
	case jobQueue <- job:
	default:
	}
}

func NewTextContent() *textContent {
	return &textContent{
		Tag: "text",
	}
}
func NewAtContent() *atContent {
	return &atContent{
		Tag: "at",
	}
}

func NewImgContent() *imgContent {
	return &imgContent{
		Tag: "img",
	}
}

func (con *textContent) SetValue(text string, attr ...interface{}) Content {
	con.Text = text
	return con
}
func (con *textContent) IsValue() bool {
	return con.Text != ""
}

func (con *atContent) SetValue(userID string, attr ...interface{}) Content {
	con.UserID = userID
	return con
}

func (con *atContent) IsValue() bool {
	return con.UserID != ""
}

func (con *imgContent) SetValue(imgPath string, attr ...interface{}) Content {
	img, err := newFileUploadRequest(imgPath)
	if err != nil {
		return con
	}
	con.Height = img.Height
	con.Width = img.Width
	con.ImageKey = img.ImageKey
	return con
}

func (con *imgContent) IsValue() bool {
	return con.ImageKey != ""
}

func sendV2(webhook, title string, con ...Content) {
	if enable := config.Get("feishu.enable").Bool(false); !enable {
		return
	}
	if webhook == "" {
		webhook = config.Get("feishu.webhook").String("")
	}
	name := config.Get("service.name").String("")
	env := config.Get("service.env").String("")
	if name != "" && env != "" {
		title = fmt.Sprintf("【%s - %s】%s", name, env, title)
	}

	var conBody = make([][]Content, 0, len(con))
	if len(con) > 0 {
		for _, v := range con {
			if v.IsValue() {
				conBody = append(conBody, []Content{v})
			}
		}
	}
	var data = msgPostBody{
		MsgType: "post",
		Content: struct {
			Post map[string]msgBody `json:"post"`
		}{
			Post: map[string]msgBody{
				"zh_cn": msgBody{Title: title, Content: conBody},
			},
		},
	}
	req := Request{TimeOut: 1 * time.Second}
	req.Request(webhook, http.MethodPost, data, nil)
}

func send(webhook, title, text string, at string) {

	enable := config.Get("feishu.enable").Bool(false)
	if !enable {
		return
	}

	//v2 存在使用v2接口
	if webhook == "" {
		webhook = config.Get("feishu.webhook").String("")
	}
	if strings.Contains(webhook, "v2") {
		con := make([]Content, 0, 2)
		con = append(con, NewTextContent().SetValue(text))
		if at != "" {
			if user, err := searchUserIDByEmail(at); err == nil {
				con = append(con, NewAtContent().SetValue(user.UserID))
			} else {
				con = append(con, NewTextContent().SetValue("@"+at))
			}
		}
		sendV2(webhook, title, con...)
		return
	}

	//v1接口
	name := config.Get("service.name").String("")
	env := config.Get("service.env").String("")
	if name != "" && env != "" {
		title = fmt.Sprintf("【%s - %s】%s", name, env, title)
	}
	var data = struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}{Title: title, Text: text}

	req := Request{TimeOut: 1 * time.Second}
	req.Request(webhook, http.MethodPost, data, nil)
}

//获取飞书操作token
func getToken(appId, appSecret string) (string, error) {
	var cacheKey = fmt.Sprintf("%x", md5.Sum([]byte(appId+appSecret)))

	val, ok := feishuCache.Get(cacheKey)
	if ok && val != nil {
		return val.(string), nil
	}
	result := struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expires           int    `json:"expire"`
	}{}
	req := Request{TimeOut: 1 * time.Second}
	if err := req.BindJSON(AccessTokenApi, http.MethodPost, map[string]string{
		"app_id":     appId,
		"app_secret": appSecret,
	}, nil, &result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", errors.New(result.Msg)
	}
	feishuCache.Set(cacheKey, result.TenantAccessToken, time.Duration(result.Expires-60*5)*time.Second)
	return result.TenantAccessToken, nil
}

//根据邮箱搜索userID， 操作需要授权
func searchUserIDByEmail(email string) (*user, error) {
	data, err := searchUserID(map[string]string{"emails": email})
	if err != nil {
		return nil, err
	}

	var res emailUserList
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}

	if res.Code != 0 {
		return nil, errors.New(res.Msg)
	}

	if v, ok := res.Data.EmailUsers[email]; ok && len(v) > 0 {
		return &(v[0]), nil
	}

	return nil, errors.New("user is not exist")
}

func searchUserIDByMobile(mobile string) (*user, error) {
	data, err := searchUserID(map[string]string{"mobiles": mobile})
	if err != nil {
		return nil, err
	}
	var res mobileUserList
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	if res.Code != 0 {
		return nil, errors.New(res.Msg)
	}

	if v, ok := res.Data.MobileUsers[mobile]; ok && len(v) > 0 {
		return &(v[0]), nil
	}

	return nil, errors.New("user is not exist")
}

func searchUserID(param map[string]string) ([]byte, error) {
	paramStr, _ := json.Marshal(param)

	var cacheKey = fmt.Sprintf("%x", md5.Sum(paramStr))
	val, ok := feishuCache.Get(cacheKey)
	if ok && val != nil {
		return val.([]byte), nil
	}

	token, err := getToken(config.Get("feishu.app_id").String(""),
		config.Get("feishu.app_secret").String(""))
	if err != nil {
		return nil, err
	}

	req := Request{TimeOut: 1 * time.Second}
	data, err := req.Request(SearchUserIDApi, http.MethodGet, param, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	})

	if err != nil {
		return nil, err
	}

	feishuCache.Set(cacheKey, data, time.Duration(60*10)*time.Second)
	return data, nil
}

type FeishuImgInfo struct {
	ImageKey string
	Width    int
	Height   int
}

//上传图片到飞书
//https://open.feishu.cn/document/ukTMukTMukTM/uEDO04SM4QjLxgDN
func newFileUploadRequest(path string) (*FeishuImgInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fileData, _ := ioutil.ReadAll(file)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", path)
	if err != nil {
		return nil, err
	}

	im, _, err := image.DecodeConfig(bytes.NewBuffer(fileData))
	if err != nil {
		return nil, err
	}

	_, _ = part.Write(fileData)
	_ = writer.WriteField("image_type", "message")

	if err := writer.Close(); err != nil {
		return nil, err
	}
	token, err := getToken(config.Get("feishu.app_id").String(""),
		config.Get("feishu.app_secret").String(""))
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest("POST", ImgUploadApi, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var uploadResp uploadImgResp
	if err := json.Unmarshal(respBytes, &uploadResp); err != nil {
		return nil, err
	}
	if uploadResp.Code != 0 {
		return nil, errors.New(uploadResp.Msg)
	}

	return &FeishuImgInfo{
		ImageKey: uploadResp.Data.ImageKey,
		Width:    im.Width,
		Height:   im.Height,
	}, err
}

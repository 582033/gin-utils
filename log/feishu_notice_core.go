package log

import (
	"fmt"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/feishu"
	"regexp"
	"strings"

	"go.uber.org/zap/zapcore"
)

type FeiShuNoticeCore struct {
	enc zapcore.Encoder
	// 附加的开关函数
	enabled func() bool
}

func NewFeiShuNoticeCore(enc zapcore.Encoder) *FeiShuNoticeCore {
	return &FeiShuNoticeCore{
		enc: enc,
	}
}
func (c *FeiShuNoticeCore) SetEnabled(f func() bool) {
	c.enabled = f
}
func (c *FeiShuNoticeCore) Enabled(level zapcore.Level) bool {
	if c.enabled != nil && !c.enabled() {
		return false
	}
	return level >= zapcore.WarnLevel
}
func (c *FeiShuNoticeCore) With(fields []zapcore.Field) zapcore.Core {
	clone := &FeiShuNoticeCore{
		enc:     c.enc.Clone(),
		enabled: c.enabled,
	}
	for i := range fields {
		fields[i].AddTo(clone.enc)
	}
	return clone
}
func (c *FeiShuNoticeCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}
func (c *FeiShuNoticeCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	// 不用实际写入任何东西，发送通知即可

	var title = fmt.Sprintf("%s %s:%d", strings.ToUpper(ent.Level.String()),
		ent.Caller.File, ent.Caller.Line)
	var at string
	if idx := strings.LastIndex(ent.Message, "@@"); idx != -1 {
		at = ent.Message[idx+2:]
		if VerifyEmailFormat(at) {
			ent.Message = ent.Message[:idx]
		}
	}
	buf, err := c.enc.EncodeEntry(ent, fields)
	if err != nil {
		return err
	}

	msg := buf.String()
	buf.Free()

	var webhook string
	switch ent.Level {
	case zapcore.DebugLevel, zapcore.InfoLevel, zapcore.WarnLevel:
		webhook = config.Get("feishu.webhook_warn").String("")
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		webhook = config.Get("feishu.webhook").String("")
	default:
		webhook = config.Get("feishu.webhook").String("")
	}
	feishu.SendV3(webhook, title, msg, at)
	return nil
}

func (c *FeiShuNoticeCore) Sync() error { return nil }

func VerifyEmailFormat(email string) bool {
	pattern := `\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*`
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(email)
}

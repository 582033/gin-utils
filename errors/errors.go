package errors

import (
	"encoding/json"
	"github.com/582033/gin-utils/util"
	"runtime"
	"strconv"
	"strings"
)

// Error implements the error object.
type Error struct {
	Code int         `json:"code,omitempty"`
	File string      `json:"file,omitempty"`
	Msg  string      `json:"msg,omitempty"`
	Err  interface{} `json:"err,omitempty"`
}

func (e *Error) Error() string {
	b, _ := json.Marshal(e)
	return util.Bytes2str(b)
}

// New generates a custom error.
func New(code int, detail string) error {
	newErr := &Error{
		Code: code,
		Msg:  detail,
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return newErr
	}
	f := strings.Split(file, "/")
	newErr.File = f[len(f)-1] + " [" + strconv.Itoa(line) + "]"
	return newErr
}

func WithMsg(e error, msg string) error {
	return newError(e, 0, msg)
}

func WithCode(e error, code int) error {
	return newError(e, code, "")
}

func WithCodeMsg(e error, code int, msg string) error {
	return newError(e, code, msg)
}

func newError(e error, code int, msg string) error {
	info := &Error{
		Code: code,
		Msg:  msg,
	}
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return info
	}
	f := strings.Split(file, "/")
	info.File = f[len(f)-1] + " [" + strconv.Itoa(line) + "]"
	if e != nil {
		if err, ok := e.(*Error); ok {
			info.Err = e
			if info.Code == 0 {
				info.Code = err.Code
			}
		} else {
			info.Err = &Error{
				Msg: e.Error(),
			}
		}
	}
	return info
}

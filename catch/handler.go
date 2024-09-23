package catch

import (
	"context"
	"fmt"
	"github.com/582033/gin-utils/apm"
	"github.com/582033/gin-utils/config"
	"github.com/582033/gin-utils/feishu"
	_ "google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/status"
	"runtime/debug"
)

func GRPC(_ context.Context, p interface{}) (err error) {
	enable := config.Get("feishu.enable").Bool(false)
	if enable {
		send(fmt.Sprintf("%s", p), string(debug.Stack()))
	}
	return status.Errorf(codes.Internal, "%s", p)
}

func send(title, text string) {
	feishu.SendV2(title, text, "")
	if meter := apm.Meter("panic", "total"); meter != nil {
		meter.Mark(1)
	}
}

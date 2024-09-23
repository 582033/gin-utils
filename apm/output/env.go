package output

import (
	"github.com/582033/gin-utils/config"
	"os"
)

var staticShortHostname, _ = os.Hostname()

func shortHostname() string {
	return staticShortHostname
}

func serviceName() string {
	return config.Get("service.name").String("")
}

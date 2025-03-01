package wintun

import (
	"os"

	"github.com/xtls/xray-core/common"
)

func init() {
	common.Must(
		os.WriteFile("wintun.dll", dllBytes, 666),
	)
}

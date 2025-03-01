package wintun

import _ "embed"

//go:embed arm64/wintun.dll
var dllBytes []byte

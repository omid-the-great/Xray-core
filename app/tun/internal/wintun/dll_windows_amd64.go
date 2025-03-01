package wintun

import _ "embed"

//go:embed amd64/wintun.dll
var dllBytes []byte

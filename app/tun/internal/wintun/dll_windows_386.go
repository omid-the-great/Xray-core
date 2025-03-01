package wintun

import _ "embed"

//go:embed x86/wintun.dll
var dllBytes []byte

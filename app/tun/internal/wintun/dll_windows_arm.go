package wintun

import _ "embed"

//go:embed arm/wintun.dll
var dllBytes []byte

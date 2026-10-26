package main

import _ "embed"

// appIcon 为 macOS 运行时提供带圆角与透明留边的图标
//
//go:embed build/darwin/appicon.png
var appIcon []byte

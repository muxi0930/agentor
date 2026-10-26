//go:build !darwin

package main

import _ "embed"

// appIcon 为 Windows 和 Linux 运行时提供应用图标
//
//go:embed build/appicon.png
var appIcon []byte

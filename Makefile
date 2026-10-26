WAILS_VITE_PORT ?= 9256
EXE_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

.PHONY: dev build build_exe build_mac build_linux icons windows_resources package_darwin package_linux package_windows

# 启动当前平台的开发模式
dev:
	wails3 dev -config "build/config.yml" -port "$(WAILS_VITE_PORT)"

# 构建当前平台的应用，嵌入前端生产资源
build:
	wails3 generate bindings -ts -i
	npm --prefix "frontend" run build
	mkdir -p "bin"
ifeq ($(GOOS),windows)
	$(MAKE) windows_resources
endif
	go build -buildvcs=false -o "bin/agentor$(EXE_EXT)" .
	$(MAKE) package_$(GOOS)

# 交叉编译 Windows x64 应用，绑定生成仍在宿主平台执行
build_exe:
	wails3 generate bindings -ts -i
	npm --prefix "frontend" run build
	mkdir -p "bin"
	$(MAKE) windows_resources GOARCH=amd64
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -ldflags="-H windowsgui" -o "bin/agentor.exe" .

# 从统一主图重新生成各平台图标
icons:
	wails3 generate icons -input "build/appicon.png" -macfilename "" -windowsfilename "build/windows/icon.ico"
	wails3 generate icons -input "build/darwin/appicon.png" -macfilename "build/darwin/icon.icns" -windowsfilename ""

# 将图标嵌入指定架构的 Windows 可执行文件
windows_resources:
	wails3 generate syso -arch "$(GOARCH)" -icon "build/windows/icon.ico" -manifest "build/windows/agentor.exe.manifest" -out "rsrc_windows_$(GOARCH).syso"

build_mac build_linux: build

# 为 macOS 应用包设置 Finder 与 Dock 图标
package_darwin:
	mkdir -p "bin/agentor.dev.app/Contents/MacOS" "bin/agentor.dev.app/Contents/Resources"
	cp "bin/agentor" "bin/agentor.dev.app/Contents/MacOS/agentor"
	cp "build/darwin/Info.dev.plist" "bin/agentor.dev.app/Contents/Info.plist"
	cp "build/darwin/icon.icns" "bin/agentor.dev.app/Contents/Resources/icon.icns"

# 按 Linux 标准安装目录组织二进制、桌面入口和图标
package_linux:
	mkdir -p "bin/agentor-linux/usr/bin" "bin/agentor-linux/usr/share/applications" "bin/agentor-linux/usr/share/icons/hicolor/1024x1024/apps"
	cp "bin/agentor" "bin/agentor-linux/usr/bin/agentor"
	cp "build/linux/agentor.desktop" "bin/agentor-linux/usr/share/applications/agentor.desktop"
	cp "build/appicon.png" "bin/agentor-linux/usr/share/icons/hicolor/1024x1024/apps/agentor.png"

package_windows:

jy_env: 
	source "$(HOME)/.go1.26.0"
	export PATH="$(PWD)/.tools/bin:$(PATH)"
	export CGO_ENABLED=1
	

build_jy: jy_env build_mac
	@

WAILS_VITE_PORT ?= 9256
EXE_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

.PHONY: dev build build_exe

# 启动当前平台的开发模式
dev:
	wails3 dev -config "build/config.yml" -port "$(WAILS_VITE_PORT)"

# 构建当前平台的应用，嵌入前端生产资源
build:
	wails3 generate bindings -ts -i
	npm --prefix "frontend" run build
	mkdir -p "bin"
	go build -buildvcs=false -o "bin/agentor$(EXE_EXT)" .

# 交叉编译 Windows x64 应用，绑定生成仍在宿主平台执行
build_exe:
	wails3 generate bindings -ts -i
	npm --prefix "frontend" run build
	mkdir -p "bin"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -ldflags="-H windowsgui" -o "bin/agentor.exe" .

build_mac: build
	mkdir -p "bin/agentor.dev.app/Contents/MacOS"
	cp "bin/agentor" "bin/agentor.dev.app/Contents/MacOS/agentor"
	cp "build/darwin/Info.dev.plist" "bin/agentor.dev.app/Contents/Info.plist"

jy_env: 
	source "$(HOME)/.go1.26.0"
	export PATH="$(PWD)/.tools/bin:$(PATH)"
	export CGO_ENABLED=1
	

build_jy: jy_env build_mac
	@
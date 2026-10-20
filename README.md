# agentor

使用 Wails v3、Go、React 和 shadcn/ui 构建的 Codex 桌面应用。实施范围和进度见 [PLAN.md](PLAN.md)。

开发入口：

```sh
./scripts/wails.sh dev
```

需要 Go 1.26、Node/npm（仅开发时）、当前平台的 Wails 开发依赖。脚本默认使用 `~/Env/GoSDK/go1.26.0/bin`；其他安装位置通过 `GO126_BIN` 指定。Wails CLI 固定为 `v3.0.0-beta.33`，首次运行会安装到项目的 `.tools/bin`。前端开发端口默认为 9256，可通过 `WAILS_VITE_PORT` 调整。

默认数据目录为系统用户配置目录下的 `agentor`。测试时可设置 `AGENTOR_DATA_DIR` 为临时目录。

当前按用户授权，开发期间暂时直接使用 CLI 的原生程序、登录状态与连接配置；整体目标完成后再处理独立账号配置。会话列表按 agentor 自己记录的 ID 筛选，不展示其他 CLI 会话；应用不改写 CLI 的连接配置。独立模式的数据目录已预留为 `agentor/codex/`。

Go 方法变更后运行 `./scripts/wails.sh generate bindings -ts -i`，不要手工修改 `frontend/bindings/`。

针对性检查：前端使用 `npm run typecheck` 和 `npm run test:state`（在 `frontend/` 运行）；Go 使用 `CGO_ENABLED=1 go test -race ./internal/codex ./internal/artifact`。真实 app-server 历史检查为显式启用，设置 `AGENTOR_CODEX_SMOKE_THREAD` 为本应用开发测试会话 ID，并在桌面退出释放写入方后运行对应测试。

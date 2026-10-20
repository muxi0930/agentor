# agentor

使用 Wails v3、Go、React 和 shadcn/ui 构建的 Codex 桌面应用。实施范围和进度见 [PLAN.md](PLAN.md)。

首次开发先安装前端依赖，再启动桌面窗口：

```sh
npm ci --prefix "frontend"
"./scripts/wails.sh" dev
```

需要 Go 1.26、Node/npm（仅开发时）、当前平台的 Wails 开发依赖。脚本默认使用 `~/Env/GoSDK/go1.26.0/bin`；其他安装位置通过 `GO126_BIN` 指定。Wails CLI 固定为 `v3.0.0-beta.33`，首次运行会安装到项目的 `.tools/bin`。前端开发端口默认为 9256，可通过 `WAILS_VITE_PORT` 调整。

默认数据目录为系统用户配置目录下的 `agentor`。测试时可设置 `AGENTOR_DATA_DIR` 为临时目录。

已实现普通聊天与富消息、图片/文件、真实审批、工作区和 Markdown 知识、控制器与步骤队列、固定 Go 动作流水线与调度、飞书单聊任务/附件/审批/产物、分项通知，以及独立账号、配额和 HTTP 余额查询。当前平台验收与外部待验证项见 PLAN.md 第 11 节。

当前按用户安排仍沿用 CLI 的原生程序、登录状态与连接配置。普通聊天列表只展示 agentor 登记的 CLI 会话，不展示其他 CLI 会话；应用不改写 CLI 的连接配置。

在设置的“账号与额度”中可新增独立 ChatGPT 或 API 账号，再点击“启用”。ChatGPT 登录由原生 Codex 发起，可继续或取消；API 使用 Responses 服务地址、模型和密钥。独立账号共用应用的 `codex/` 会话库，各自保留身份和模型/权限偏好。切换身份前需结束或取消活动任务和排队请求；可随时在空闲时切回 CLI。CLI 与独立模式使用不同会话库，不迁移 CLI 历史，既有工作区专用上下文可在对应页面重建。

配额取自上游实际窗口。API 余额需配置自己的 GET/POST 接口、密钥请求头和 JSON Pointer 字段，例如 `/data/balance`；POST JSON 中的完整字符串值 `$API_KEY` 引用所属账号密钥。缺失数值保持未知，查询失败不影响聊天。

飞书在“飞书连接”页填写应用配置、允许的联系人或群；群消息需 @ 当前机器人。订阅 `im.message.receive_v1` 与 `card.action.trigger`，开通消息、资源、表态和上传所需权限。测试消息只能发送给允许的联系人；桌面审批和产物接收者通过一次性 `/pair` 配对。明确失败项可重试，未知送达项需要先人工确认。

本轮验收数据与飞书配置保存在独立测试目录。需要打开同一份验收现场时运行：

```sh
AGENTOR_DATA_DIR="/private/tmp/agentor-desktop-check" "./scripts/wails.sh" dev
```

该路径是临时测试数据，正式使用可在默认数据目录重新配置。测试账号 `Local API Check` 指向已停止的本机测试服务；测试余额不代表平台资金，当前活动连接已恢复 CLI。没有待执行队列或启用中的测试调度。

Go 方法变更后运行 `"./scripts/wails.sh" generate bindings -ts -i`，不要手工修改 `frontend/bindings/`。

针对性检查：前端在 `frontend/` 运行 `npm run typecheck` 和 `npm run test:state`；Go 1.26 环境运行 `CGO_ENABLED=1 go test -race ./internal/...`。`AGENTOR_CODEX_ACCOUNTS_SMOKE=1` 显式启用隔离的原生账号启动与登录取消检查；它不完成浏览器认证。真实历史检查使用 `AGENTOR_CODEX_SMOKE_THREAD`，仅指定本应用测试会话，并先退出桌面释放写入方。

本轮只验收当前 macOS，未开展跨平台发布构建。独立 ChatGPT 完整登录、独立真实 API 推理/余额、群聊多人审批和真实外部 MCP 请求仍需相应账号与测试条件；详细记录见 PLAN.md。

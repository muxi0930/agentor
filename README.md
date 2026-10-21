# agentor

使用 Wails v3、Go、React 和 shadcn/ui 构建的 Codex 桌面应用。实施范围和进度见 [PLAN.md](PLAN.md)。

首次开发先安装前端依赖，再启动桌面窗口：

```sh
npm ci --prefix "frontend"
make dev
```

需要提前安装 Go 1.26、Node/npm、Make、Wails CLI `v3.0.0-beta.33` 和当前平台的 Wails 开发依赖，并将命令加入 PATH。构建命令不安装工具、不设置本机专用路径。前端开发端口默认为 9256，可通过 `make dev WAILS_VITE_PORT=端口号` 调整。`scripts/` 仅供本地使用，不纳入版本控制。

```sh
make build      # 构建当前平台应用，输出到 bin/
make build_exe  # 交叉编译 Windows x64，输出 bin/agentor.exe
```

两个构建命令都会先生成绑定并构建前端生产资源，再将资源嵌入应用。Windows 构建使用 `CGO_ENABLED=0`，无需额外的 C/C++ 交叉编译工具链；生成的应用运行时仍需目标平台的 Codex 原生程序。

默认数据目录为系统用户配置目录下的 `agentor`。测试时可设置 `AGENTOR_DATA_DIR` 为临时目录。

已实现普通聊天与富消息、图片/文件、真实审批、工作区和 Markdown 知识、控制中心与步骤队列、固定 Go 动作流水线与调度、飞书单聊任务/附件/审批/产物、分项通知，以及独立账号、配额和 HTTP 余额查询。当前平台验收与外部待验证项见 PLAN.md 第 11 节。

账号与 Codex 数据目录分别选择。设置中的“共用系统默认 CODEX_HOME”使用启动环境的 `CODEX_HOME`，未设置时使用 `~/.codex`；“自定义 CODEX_HOME”允许选择其他目录。“读取非本应用创建的会话”默认不勾选；勾选后会显示所选目录中其他应用创建的会话，并按目录归组，保存后立即生效，无需重启。该选项同时适用于默认目录和自定义目录。切换账号不改变目录；沿用现有配置时不改写连接配置，启用独立账号时会将其配置与文件凭证写入所选目录，其他共用该目录的客户端也可能使用这些配置。

在设置的“账号与额度”中可新增独立 ChatGPT 或 API 账号，再点击“启用”。ChatGPT 登录由原生 Codex 发起，可继续或取消；API 使用 Responses 服务地址、模型和密钥。账号共用所选目录的会话库，各自保留身份和模型/权限偏好。切换身份前需结束或取消活动任务和排队请求。切换目录保留当前账号选择，不迁移历史会话；点击“应用并重启”后需再次确认，随后复用“完全退出”的逻辑，中断运行项、取消排队项，完成关闭后重新启动整个 agentor。已有独立账号首次升级时保留原应用 `codex/` 目录。

配额取自上游实际窗口。API 余额需配置自己的 GET/POST 接口、密钥请求头和 JSON Pointer 字段，例如 `/data/balance`；POST JSON 中的完整字符串值 `$API_KEY` 引用所属账号密钥。缺失数值保持未知，查询失败不影响聊天。

侧栏底部显示当前账号、连接状态与上游已用额度，可向上展开切换账号。设置底部的“完全退出”会先列出运行和排队任务；确认后中断运行项、取消全部排队项，等待后台收尾后关闭程序。被取消的排队项不会在下次启动恢复。

飞书在“飞书连接”页填写应用配置、允许的联系人或群；群消息需 @ 当前机器人。订阅 `im.message.receive_v1` 与 `card.action.trigger`，开通消息、资源、表态和上传所需权限。测试消息只能发送给允许的联系人；桌面审批和产物接收者通过一次性 `/pair` 配对。明确失败项可重试，未知送达项需要先人工确认。

本轮验收数据与飞书配置保存在独立测试目录。需要打开同一份验收现场时运行：

```sh
AGENTOR_DATA_DIR="/private/tmp/agentor-desktop-check" make dev
```

该路径是临时测试数据，正式使用可在默认数据目录重新配置。测试账号 `Local API Check` 指向已停止的本机测试服务；测试余额不代表平台资金，当前活动连接已恢复 CLI。没有待执行队列或启用中的测试调度。

Go 方法变更后运行 `wails3 generate bindings -ts -i`，不要手工修改 `frontend/bindings/`。

针对性检查：前端在 `frontend/` 运行 `npm run typecheck` 和 `npm run test:state`；Go 1.26 环境运行 `CGO_ENABLED=1 go test -race ./internal/...`。`AGENTOR_CODEX_ACCOUNTS_SMOKE=1` 显式启用隔离的原生账号启动与登录取消检查；它不完成浏览器认证。真实历史检查使用 `AGENTOR_CODEX_SMOKE_THREAD`，仅指定本应用测试会话，并先退出桌面释放写入方。

本轮只验收当前 macOS，未开展跨平台发布构建。独立 ChatGPT 完整登录、独立真实 API 推理/余额、群聊多人审批和真实外部 MCP 请求仍需相应账号与测试条件；详细记录见 PLAN.md。

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

应用图标使用白底蓝色 A，Windows/Linux 主图为 `build/appicon.png`；macOS 使用 `build/darwin/appicon.png`，带透明留边和圆角白底，使 Dock 中的视觉尺寸与其他应用一致。执行 `make icons` 可重新生成 macOS 的 ICNS 和 Windows 的多尺寸 ICO。`make build` 在 macOS 上同时生成带图标的 `bin/agentor.dev.app`，Windows 原生构建和 `make build_exe` 会将图标嵌入 EXE。Linux 原生构建同时生成 `bin/agentor-linux/usr/` 安装目录，将其中内容安装到 `/usr/` 或 `/usr/local/` 对应目录后，应用菜单使用配套的桌面入口和图标；直接运行裸二进制不提供文件管理器图标。`make build_mac`、`make build_linux` 分别是对应平台的本机构建入口，需要在目标平台执行。

应用启动时按以下优先级确定数据根目录：进程环境变量 `AGENTOR_DATA_DIR` > `~/.config/agentor/conf.yaml` 中的 `AGENTOR_DATA_DIR` > 系统用户配置目录下的 `agentor`。未设置环境变量且配置文件不存在时，会自动生成默认配置；环境变量覆盖时不会创建或改写全局配置文件。例如：

```yaml
AGENTOR_DATA_DIR: /absolute/path/to/agentor
```

全局配置固定放在用户主目录下的 `.config/agentor/conf.yaml`，Windows 同样如此，例如 `C:/Users/用户名/.config/agentor/conf.yaml`，不使用 `AppData` 保存此文件。默认数据根目录仍沿用各平台原来的系统配置目录。

设置页的“应用数据根目录”支持选择目录，经二次确认后保存并完整重启。切换后读取目标目录的数据，不自动迁移当前设置、账号、任务和附件；如需保留，请先退出应用并自行拷贝。原目录文件不会被删除。设置了环境变量时，请先移除该变量并重启，再通过界面切换目录。测试时可设置 `AGENTOR_DATA_DIR` 为临时目录。

已实现普通聊天与富消息、图片/文件、真实审批、工作区和 Markdown 知识、控制中心与步骤队列、Go 状态机驱动的静态 / Agent 节点流水线与调度、飞书单聊任务/附件/审批/产物、分项通知，以及独立账号、配额和 HTTP 余额查询。当前平台验收与外部待验证项见 PLAN.md 第 11 节。

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

### 流水线模板、实例与节点图

侧栏「流水线模板」提供公共模板和无需项目的独立流水线。项目内可直接新建流水线，或应用模板并填充预设的项目步骤插槽。应用模板会复制节点图；实例可独立编辑，后续模板修改不影响已创建的实例。

画布参考 Demo 的 Diagram ASCII Flow 简洁节点交互，底层采用 React Flow（@xyflow/react）。从侧栏拖入或点击添加节点，从分支出口拖动连线；点击节点，在画布内覆盖面板配置。节点坐标随定义保存。输入表单、节点配置和运行详情使用现有组件主题。

节点分为 Go 静态动作与 `agent.run · 执行 Agent 任务`，并有 `end.success` / `end.failure` 两种显式终点。Go 状态机每次仅执行一个节点，支持多个命名业务分支，每次选中一个；不支持并行和循环连线。保存时校验入口、可达性、无环、分支契约与数据依赖。

- 静态动作不重试。预期业务结果走对应出口；非预期错误或未连接的出口直接终止运行。
- Agent 配置提示词、验收标准、权限（只读、工作区写入或完全访问）、超时和 `maxRetries`。默认工作区写入；完全访问需在节点中显式选择。重试次数是额外尝试次数；2 次重试最多执行 3 次。重试耗尽后走 `failureTarget`，未配置则失败终止；业务分支不触发重试。
- 最终状态由到达的终点决定，因此失败处理分支可以在处理成功后抵达成功终点。
- 用户停止优先于重试和失败处理。暂停在节点边界生效，继续时沿保存的位置推进；失败和中断的静态动作不会重新执行。
- 每次运行冻结实例、输入、工作目录及绑定步骤上下文。已中断运行在重启后标记中断，不自动重放副作用；暂停运行保留位置。

Agent 必须调用宿主工具 `pipeline_complete_node`：

```json
{"outcome":"success","branch":"ready","summary":"验收依据","output":{"answer":"任意 JSON 或文本"}}
```

宿主从实际 thread/turn 确定 run/node/attempt；Agent 不传下一节点 ID。仅接受已声明分支；同一汇报可重复确认但不能覆盖。汇报可靠保存且原生轮次正常结束后，Go 才推进。没有汇报、验收失败、超时或异常轮次按 Agent 重试策略处理。失败汇报使用 `outcome=failure`、空 `branch`。普通聊天文本及文件不会推进状态机；文件可以作为输出中的交付物路径。`output` 通过 Go 保存并传给后续节点，输入支持嵌套 `{ "input": "名称" }` 和 `{ "result": "节点ID", "path": "对象.字段" }`，省略 path 引用完整值。结果引用必须来自所有到达路径上的必经前序节点。

绑定项目步骤的 Agent 尝试同步出现在步骤执行记录，只有验收通过且轮次正常结束时标记步骤完成。Agent 节点与普通步骤共用全局并发上限。管理与配置专用会话已移除，控制中心直接调用保存、应用模板、运行、暂停、继续、停止和定时触发等宿主接口；节点 Agent 只有结果汇报工具。

Git 链路可编排为 `worktree.create → agent.run → worktree.merge → worktree.remove → end.success`。创建时保存来源分支与提交；Agent 在独立工作区执行并自行提交需合并的变更。合并只允许快进，未提交变更返回 `dirty`，分支分歧返回 `diverged`，可按需连接处理节点；没有对应连线时终止并保留现场。清理只删除已确认合并、没有新增提交的 worktree 目录和分支。

版本级工作区通过控制中心的 `prepare_version_worktree`、`get_version_worktree`、`merge_version_worktree` 操作。例如“为版本 V1 创建 worktree，后续在里面开发”和“检查并合并版本 V1 的 worktree”。两条内置系统流水线可查看但不可编辑、删除、复制或配置定时触发，只能通过版本入口启动；运行历史和 Agent 会话继续显示在现有监控中。

创建时以项目当前分支和提交作为来源，不要求指定目标分支，后续合回记录的来源分支。版本绑定独立持久化，重启后保留。普通派发和流水线绑定步骤均由 Go 确定执行目录，绑定异常会停止，不回退主目录；没有绑定的版本仍使用项目目录。同一功能内步骤串行，不同功能仍允许并行，共用同一版本工作区。

准备或合并期间阻止该版本的新执行及结构变更。合并要求版本包含功能和步骤、全部完成，且无排队、运行或暂停的相关任务。两侧存在未提交改动时停止并提示；分叉时由 Agent 在版本 worktree 内整合目标分支、解决冲突，并按仓库现有说明执行相关测试和构建，在节点报告中列出验证命令和结果，验证不通过则停止。宿主重新检查 Git 状态后执行快进合并，成功后自动删除 worktree 和工作分支。清理失败保留“已合并，待清理”记录，再次发出合并指令只重试清理。侧边栏版本节点的树状图标提示工作区目录，版本详情保留合并提交和最近运行记录。

定时触发支持一次性与固定间隔、启停及错过后补一次/跳过，直接启动实例；同一实例禁止重入。仅在应用进程运行时调度。数据写入 `pipelines/graph-state.json`，本版不迁移或加载旧的顺序流水线 `state.json`。

验证：`go test ./internal/pipeline ./internal/toolrouter ./internal/tasks ./internal/lifecycle ./internal/lark`；前端 `npm run typecheck`、`npm run build`、`npm run test:pipeline`。画布测试使用独立 Chrome 临时资料目录，协议测试使用本地原生进程模拟器，不连接模型或发送外部消息。`TestNativePipelineWorktree` 仍提供显式开启的真实模型链路实测。

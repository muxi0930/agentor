# agentor 开发与交接计划

计划日期：2026-10-20  
状态：执行中，P0/P1 已完成，P2 完整聊天与账号/审批开发中  
目标目录：`/Users/zhui/Work/p/wails/agentor`

本计划将 CodexPFWork 的业务功能重新实现为 agentor，工程结构参考 demo。它同时作为功能清单、实现约定、开发排期和新会话的交接入口。包含 12 类、114 项功能，分为 8 个开发阶段。后续编码不需要依赖此前聊天记录。

## 1. 已确认的要求

| 项目 | 执行约定 |
| --- | --- |
| 参考源码 | `/Users/zhui/Work/p/wails/CodexPFWork` |
| 参考架构 | `/Users/zhui/Work/p/wails/demo` 的 Wails v3、Go service、React、TypeScript、Vite、Tailwind 和 shadcn 组织方式。 |
| 应用定位 | 全新应用、全新数据目录、全新账号配置；不迁移旧数据，不兼容旧文件或旧接口。 |
| 运行时 | Wails + Go；应用不携带 Node，不启动 Node 后端、Node 脚本或 Electron。 |
| 开发工具 | 用户已确认可以使用 Node/npm/Vite/shadcn CLI 开发和编译前端。浏览器中的 JavaScript 仍是 React 界面所需。 |
| 流水线 | 用户已选择“固定 Go 动作 + JSON 任务配置”；不动态编译 Go 源码，也不执行 JavaScript 脚本。 |
| UI | 每个 UI 需求先查 shadcn 现成组件，其次组合已有组件，确实缺少能力时才自行封装。 |
| 后端 | 简单、可读，按真实功能拆分；不增加通用框架、推测性抽象或层层防护。 |
| 平台 | 按跨平台方式编写路径、进程和桌面交互；本轮只验证当前机器上的运行时。 |
| 验收 | 少量关键业务流程即可；不要求构建各平台可执行文件、安装包、签名或公证。 |
| 本次交付 | 仅落地计划文档。应用编码交给后续新会话执行。 |

“不保留旧依赖”指不继承 Electron 工程及其依赖树。新应用仍可以按实际需要使用 React、shadcn、Markdown/Mermaid 渲染库和官方 Go SDK，并分别锁定版本。

## 2. 产品范围与调整

### 2.1 保留的业务能力

1. Codex 普通对话：项目目录、会话列表、模型选择、图片输入、流式消息、历史消息、工具过程、停止与追加指令。
2. 账号与设置：ChatGPT/API 账号、切换、连接设置、额度或余额、权限模式。
3. 工作区：项目、版本、功能、步骤、上下文与提示词、排序、标签、会话绑定。
4. 知识文档：导入、预览、绑定、搜索、读取、由 Agent 创建或更新。
5. 控制器：接收请求、查找工作对象、创建和派发步骤、查询状态、停止任务。
6. 流水线：配置会话、管理会话、固定动作、JSON 任务、手动运行、定时运行、运行记录。
7. 飞书：连接、授权联系人/群、接收任务、附件输入、结果回复、审批卡片。
8. 附件、审批与通知：任务附件归属、产物分享、真实审批、问答、通知记录与失败重试。

### 2.2 有意重新设计的部分

| 原方案 | agentor 方案 | 对功能的影响 |
| --- | --- | --- |
| Electron main/preload/IPC | Wails Go service + 自动生成的 TypeScript bindings + 事件 | 业务保留，通信和桌面能力重写。 |
| Node 子进程运行 `entry.js` | 固定 Go 动作执行 JSON 配置 | AI 可以编排已有动作，不能自行增加可执行代码。新增动作需要修改 agentor 的 Go 源码。 |
| 流水线 build 会话编写脚本 | “配置会话”编写任务和定时配置 | 保留 AI 辅助配置，取消脚本源码、解释器、编译器管理。 |
| JS 余额脚本和 QuickJS 沙箱 | Go HTTP 请求 + JSON 字段映射 | 支持常见余额接口；任意 JavaScript 算法、HTML 抓取和多步脚本登录不在本版范围。 |
| 飞书 CLI 子进程 | 飞书官方 Go SDK 的 API 和长连接 | 不安装或启动 lark-cli，不解析 CLI 的 NDJSON。 |
| 与其他工具共享、合并已有 Codex 配置 | agentor 独立的应用数据目录和 `CODEX_HOME` | 用户在新应用中重新登录或添加账号；不读取其他应用的账号及历史。 |
| 桌面直发步骤与控制器派发走不同路径 | 两种入口共用同一套步骤执行流程 | 每次正式派发都有 Execution 记录和统一上下文。 |
| 重启后复杂的孤儿任务修复与自动重放 | 运行中任务标记为 interrupted；排队任务等待用户恢复 | 不自动重放结果不明的执行，不制作历史修复工具。 |

上述调整属于本计划的明确范围，不应在编码时悄悄恢复旧方案。

### 2.3 不实施的内容

- 旧账号导入、CC Switch 导入、旧 `CODEX_HOME` 合并、旧通知/知识库/工作区迁移。
- 旧 JSON 字段、旧 app-server 协议分支、旧 dynamic tools 版本的兼容处理。
- Node 运行器、QuickJS、旧 Electron 预加载脚本、旧脚本测试工具。
- G004/Unity、固定盘符、指定工作仓库、旧 Python/PowerShell 工具和其他项目专用规则。
- 为兼容历史异常构建的多代备份、校验和、跨进程写锁、CAS 修复协议、自动重复执行。
- 任意脚本插件、动态 Go 编译、通用 DAG、条件表达式语言、分布式队列、向量知识库。
- 系统托盘、自动更新、交互式 PTY 终端、全局搜索等原项目未形成实际业务闭环的扩展。
- 跨平台发布构建、安装包、签名、公证、商店发布、性能压测和大规模端到端测试工程。

源码中只有配置字段、没有实际调用链的能力，不能写成已完成的业务功能。例如原有 skillSettings 不等于完整的技能安装与执行系统，原有 speed 设置不等于已经实现了加速模式。

## 3. 技术方案与简单实现边界

### 3.1 工程基线

以本次读取的 demo 为基线：Go `1.26.0`、Wails `v3.0.0-beta.33`、React `19.3.1`、TypeScript `7.0.2`、Vite `8.3.4`、Tailwind `4.3.3`、shadcn CLI `4.22.1`。shadcn 使用 demo 的 `base-nova`、Base UI、neutral、CSS variables 和 lucide 配置。

只引入 agentor 用到的依赖和组件。demo 中的展示页、展示数据、图表库和测试工程不整体复制。新建 agentor 自己的 module、package 信息和依赖锁文件，不继承 CodexPFWork 的 package.json。

Wails bindings 自动生成，不手工编辑。前端不另设 HTTP 后端；普通业务请求直接调用 Go service，流式内容使用 Wails 事件。需要提供图片或隔离 HTML 资源时，使用 Wails 资源处理能力，不额外启动 Node 或 Web 服务。

Wails service 的绑定与生命周期方式参考[官方 Services 文档](https://v3.wails.io/features/bindings/services/)。具体 API 以 demo 锁定版本的实际代码为准，不把最新文档中的 API 直接假定为当前版本可用。

### 3.2 运行依赖

| 依赖 | 用途 | 处理方式 |
| --- | --- | --- |
| 系统 WebView | React UI 的桌面容器 | 使用 Wails；当前平台实际启动验证。 |
| Codex 原生可执行文件 | 启动 app-server，提供模型、工具和会话能力 | 设置中指定可执行文件，或从 PATH 查找；不通过 `npx`、Node 包入口或旧应用缓存目录启动。 |
| Git | 创建 worktree | 用户触发此功能时使用本机 Git，缺少时展示明确原因。 |
| 飞书账号和应用配置 | 远程任务及通知 | 用户主动配置后启用，通过 Go SDK 连接。 |
| Node/npm、Go 工具链 | 开发 agentor | 属于开发环境；最终应用不因流水线功能额外依赖这些工具链。 |

agentor 仍是 Codex app-server 的桌面宿主，不重新实现 LLM Agent 循环。用户任务所需的项目工具由用户项目决定，与 agentor 自身是否依赖 Node 分开看待。

### 3.3 目录与职责

以下是目标组织方式，按里程碑逐步创建，禁止先搭一批空模块。

```text
agentor/
  PLAN.md
  main.go                     # Wails 装配、窗口、生命周期
  tools.go                    # 当前实际使用的 dynamic tools 路由装配
  go.mod
  Taskfile.yml
  scripts/wails.sh             # 参考 demo 的本机开发入口
  build/                      # 模板所需开发配置；不开展发布打包
  internal/
    appdata/                  # 数据根目录、少量 JSON 读写助手
    codex/                    # app-server、会话、账号、权限与上游审批
    workspace/                # 项目/版本/功能/步骤定义、知识文档
    tasks/                    # 控制器、Execution、队列、结果与通知记录
    pipeline/                 # Go 动作、JSON 配置、调度、运行记录
    artifact/                 # 输入附件、产物、文件资源访问
    lark/                     # Go SDK 接入、路由、收发、卡片
  frontend/
    bindings/                 # 自动生成
    src/
      app/                    # 外壳、导航、主题、事件订阅
      components/ui/          # shadcn 组件
      features/
        chat/
        accounts/
        workspace/
        knowledge/
        controller/
        pipeline/
        lark/
        approvals/
        notifications/
      hooks/                  # 已有多个实际使用方的 hook
      lib/                    # 少量已被实际使用的工具
```

每个 Go 包可以按功能分成几个文件，不为 CRUD 建立 controller/service/repository 三层转发。模块之间优先直接使用具体类型；遇到实际循环依赖时，由装配处注入所需函数，不提前建立通用插件总线。

`codex` 处理上游协议，不知道飞书；`workspace` 处理工作定义，不启动任务；`tasks` 负责正式执行；`pipeline` 复用 tasks 执行步骤；`lark` 把远程输入交给已有流程。dynamic tools 的角色分发放在装配处，避免让每个模块互相导入。

### 3.4 数据与状态归属

- Go 持有业务状态：账号、工作区、任务、队列、审批、附件、定时任务、通知。
- React 持有界面状态：当前页面、展开项、输入草稿、弹窗、滚动位置。
- 会话正文以 agentor 自己的 Codex app-server 为准，不另建完整聊天数据库。
- 应用数据使用系统用户数据目录下的 `agentor`，通过 Go 跨平台路径 API 构造；不硬编码 `/Users/...` 或 Windows 盘符。
- agentor 内的账号共用专属 `CODEX_HOME` 和会话库。切换账号切换连接身份，不导入其他应用的数据。
- 使用小型 JSON 文件保存设置、工作区和任务记录，知识文档及附件独立保存。不引入数据库、ORM 或通用 repository。
- 同一份状态只有一个 Go 写入方；共享可变状态使用必要的 mutex，写文件采用同目录临时文件替换。无需跨进程锁和多代回滚系统。
- 密钥不出现在列表 DTO、事件、日志和聊天上下文。认证文件使用当前用户可访问的应用目录；不自制加密协议，不复刻 Electron 的凭据存储兼容层。

推荐数据分组：`settings.json`、`accounts.json`、`codex/`、`workspace.json`、`tasks/`、`pipelines/`、`knowledge/`、`attachments/`、`notifications/`。这是新格式，不包含旧版格式迁移入口。

### 3.5 必要状态机

| 对象 | 状态 | 关键约定 |
| --- | --- | --- |
| Runtime | stopped / starting / ready / error | 未 ready 不发送任务；重连失败给用户可见错误。 |
| Request / Execution / PipelineRun | queued / running / completed / error / interrupted | 请求、步骤执行、流水线运行各自记录，不用一个状态覆盖全部对象。 |
| Approval | pending / resolved / expired | 上游请求结束或连接失效后不可继续批准。 |
| Attachment | pending / ready / failed | 保留所属请求/步骤；失败显示原因。 |
| NotificationPart | pending / sent / failed / unknown / ignored | 业务执行成功与通知发送成功分开；状态不明不自动重发。 |

运行期每个步骤串行，不同步骤可以并行；控制器请求按 FIFO 顺序处理。模型与 effort 在入队时固定，后续切换 UI 选项不改变已排队任务。

应用重启后，原 running 状态转为 interrupted；queued 记录保留，用户点击恢复后继续。定时任务恢复遵守自己的 skip/fire_once 配置，不能据此重放原本已经在执行的任务。

保留停止、超时、消息去重、归属校验等实际影响正确性的逻辑。不要增加全站统一重试框架、复杂补偿事务或每一层重复校验。

## 4. UI：必须先查 shadcn

### 4.1 每个 UI 需求的开发顺序

1. 查 demo 的 `frontend/src/features/showcase/catalog.ts` 和 `frontend/src/components/ui/`，确认已有组件与行为。
2. 查 shadcn 官方对应的 Base UI 组件、示例或组合方式。
3. 直接使用或组合组件；缺少的只是业务数据时，在 `features/` 中写业务组件。
4. 只有官方组件没有对应交互能力时才补充封装，并在交付说明用一句话说明缺口。

不额外制作通用按钮、表单、弹窗、通知、侧栏或聊天气泡系统。不要同时引入第二套 UI 框架。

### 4.2 组件对应表

| agentor 界面需求 | 优先使用 | 需要自己实现的部分 |
| --- | --- | --- |
| 主侧栏、模式切换、设置入口 | Sidebar、Tabs、DropdownMenu、Tooltip | 业务导航与当前项。 |
| 项目/版本/功能/步骤层级 | Sidebar、Collapsible、Item、Badge、ContextMenu | 树形数据及选中/展开规则。 |
| 功能卡片与版本区域 | Card、Badge、ScrollArea、Empty | 数据分组、排序与横向滚动。 |
| 表单、提示词、设置字段 | Field、Input、Textarea、Select、Switch | 业务字段、提交与错误提示。 |
| 新建/编辑/删除入口 | Dialog、Sheet、AlertDialog | 业务内容；普通编辑不增加多次确认。 |
| 模型/账号选择 | Combobox 或 Select | 选项数据、模型能力与账号状态。 |
| 消息布局和气泡 | Message、Bubble、MessageScroller | 消息分类、流式合并、历史加载。 |
| 工具过程与思考摘要 | Trace、Collapsible、Marker | Codex 事件到过程节点的映射。 |
| 输入框与附件栏 | Textarea、InputGroup、Attachment、Button | Enter/Shift+Enter、粘贴、发送状态。 |
| 图片/文件展示 | Attachment、AspectRatio、Dialog | 图片预览与文件资源地址。 |
| 代码块 | CodeBlock | 语言识别、复制和内容加载。 |
| 文件 diff | CodeBlock、Tabs、ScrollArea | 行号、增删行与文件导航，作为业务视图实现。 |
| Mermaid | 现有展示容器 | Mermaid 渲染与失败时源码展示。 |
| HTML 可视化 | Card、Skeleton、Alert | 独立 iframe、尺寸、加载与后续提问消息。 |
| 额度/余额 | Progress、Badge、Card、Tooltip | 数值和状态映射；无需先引入 Chart。 |
| 审批中心 | Sheet、Card、Badge、AlertDialog | 请求类型、来源、允许/拒绝动作。 |
| 多项用户问答 | Questionnaire | 请求字段映射、答案提交。 |
| 通知中心 | Sheet、Item、Badge、Button | 发送状态、失败原因、重试入口。 |
| 任务/运行列表 | Table；确有排序筛选需求时再用 DataTable | 行数据、状态和详情入口。 |
| 一次性/间隔调度表单 | DatePicker、Input、Select、Switch | 时区、间隔与错过触发策略。 |
| 排序拖拽 | 先查官方示例与现有组件组合 | 确有缺口再增加一个拖拽实现，不自建拖拽框架。 |
| 主题与空白/等待/错误状态 | demo 主题、Empty、Skeleton、Spinner、Alert、Toast | 业务文案与重试动作。 |

demo 已包含 Message、Bubble、Trace、Attachment、MessageScroller 和 Questionnaire，无需从零设计聊天基础组件。官方说明分别见 [Message](https://ui.shadcn.com/docs/components/base/message)、[Trace](https://ui.shadcn.com/docs/components/base/trace)、[Questionnaire](https://ui.shadcn.com/docs/components/base/questionnaire)。

## 5. 功能清单与逐项实现思路

编号用于排期、验收和后续交接。下表均为待实现功能；“已有”仅指参考项目已有行为，不表示 agentor 已完成。

### A. 应用外壳与运行环境

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| A01 | 主窗口、布局与主题 | 复用 demo 外壳方式；建立侧栏、内容区、设置入口及浅/深色主题。 | P0 |
| A02 | Codex/工作区 模式切换 | Tabs 或侧栏导航，保存最后进入的模式及选中对象。 | P0/P3 |
| A03 | 单实例与退出清理 | 使用 Wails 单实例能力；退出时取消 goroutine、关闭长连接和 app-server。 | P0/P1 |
| A04 | 目录选择、文件选择、打开目录和外链 | 调用 Wails/系统能力；所有路径由 Go 处理。 | P0/P1 |
| A05 | Codex 程序定位与诊断 | 指定原生二进制路径或 PATH 查找；展示路径、连接状态和错误原因。 | P1 |
| A06 | 数据目录和设置持久化 | 新建独立数据目录；首次空白状态可直接使用，不探测旧应用数据。 | P0 |
| A07 | 手动重连与运行状态 | 设置页提供重连；执行前检查 ready；连接切换时结束旧 pending 请求。 | P1 |
| A08 | 当前平台开发入口 | 参考 demo 的 Wails dev 脚本和 Go 工具链路径；只确保开发态真实桌面窗口运行。 | P0 |

### B. Codex 连接、项目和会话

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| B01 | app-server 双向通信 | Go 启动原生进程，stdio JSON-RPC；请求 ID 对应结果，通知流转成 Wails 事件。 | P1 |
| B02 | 初始化与服务器请求 | 完成 initialize/initialized；支持审批和 dynamic tool 请求，保留当前协议需要的字段。 | P1/P2 |
| B03 | 会话列表与工作目录分组 | 使用上游当前列表接口；按 cwd 分组，应用只保存展示别名、置顶和排序。 | P1 |
| B04 | 新建、选择和恢复会话 | thread start/read/resume；先建立会话再发送首轮消息。 | P1 |
| B05 | 会话标题和项目展示名 | 会话标题调用上游能力；项目展示名保存本地，不改实际目录。 | P1 |
| B06 | 归档会话、归档项目下会话、移除侧栏分组 | 按动作范围更新上游或本地元数据；移除分组不删除仓库文件。 | P1 |
| B07 | 列表分页与折叠 | 保留按需“显示更多”，可用原先每批 6 条的交互；折叠后恢复初始数量。 | P1 |
| B08 | 置顶、排序与拖拽 | 保存稳定 ID 的顺序；在业务层实现拖动与边缘滚动。 | P2 |
| B09 | Git worktree | 从当前 HEAD 创建用户指定分支和目录；Go 用参数数组调用 Git，返回真实错误。 | P2 |
| B10 | 多会话并行及事件归属 | 事件携带 threadId/turnId/itemId；切换页面不改变后台任务，旧连接事件不写入新连接状态。 | P1 |

### C. 输入、消息呈现与历史

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| C01 | 输入、换行、草稿、失败恢复 | Textarea 自动增高，Enter 发送、Shift+Enter 换行；按会话保留草稿，失败恢复输入和附件。 | P1 |
| C02 | 模型与思考强度 | 从当前 model/list 构建选项，只显示模型支持的 effort；按账号保存选择。 | P1 |
| C03 | 开始、追加指令、停止 | 对应 turn/start、steer、interrupt；追加携带实际 activeTurnId，停止后使用上游最终状态。 | P1 |
| C04 | 本地图片选择、粘贴和预览 | Attachment 展示缩略图；选择文件走 Go；粘贴优先使用 WebView paste 事件，按模型能力启用。 | P2 |
| C05 | Markdown、代码、表格和链接 | 使用成熟 Markdown 渲染库，代码交给 CodeBlock；外链交给系统浏览器。 | P1/P2 |
| C06 | Mermaid 与失败展示 | 浏览器内渲染图表，失败时可查看/复制原始源码。 | P2 |
| C07 | 思考、过程与最终回答分区 | 使用 Trace/Collapsible；过程可收起，最终回答独立；结束事件统一更新节点。 | P2 |
| C08 | 命令、输出、退出码和耗时 | 映射上游 command/tool 事件，合并同一工具调用的增量；展示普通输出，不做交互终端。 | P2 |
| C09 | 文件修改与 diff | 文件列表显示增删数，详情显示上下文行与改动；只实现业务 diff 视图。 | P2 |
| C10 | 搜索、图片查看和其他工具输出 | 对已知工具使用明确视图，未知当前工具用通用结果卡，保留原始文本入口。 | P2 |
| C11 | 历史分页与大消息 | 使用当前上游分页能力，按需加载；长内容先摘要，详情再展开；保留少量最近会话缓存。 | P2 |
| C12 | 流式滚动与去重 | 增量按帧批量提交 UI；用户向上滚动暂停跟随；切换会话取消旧订阅并避免重复请求。 | P2 |
| C13 | 用量、耗时、压缩和错误状态 | 显示上游实际 token/context 数据、压缩状态和用时；缺失值显示未知，不推算虚假上限。 | P2 |
| C14 | HTML 可视化产物 | 由 Go 提供当前会话的 HTML 资源，使用隔离 iframe；支持加载/错误、宽度、高度及后续提问。 | P2 |
| C15 | 原文、附件与复制 | 文本按用户可读形式展示，附件单独列出；原始内容可查看和复制，不把内部附件标记混入正文。 | P2 |

图片粘贴是需要在当前 WebView 中尽早验证的点。[Wails Clipboard 文档](https://v3.wails.io/features/clipboard/basics/)仍列出原生剪贴板仅支持文本，因此不能用 Wails 文本 API 冒充图片支持。P2 先验证 paste 事件；只有实际失败才为当前问题添加最小平台处理。

HTML 预览中的脚本不能获得 Wails 绑定或读取宿主数据；iframe 消息只处理尺寸和用户明确触发的后续提问。这是执行生成 HTML 的必要边界，不建设通用浏览器沙箱平台。

### D. 账号、连接配置与余额

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| D01 | ChatGPT/API 账号列表和新增 | 本地保存账号 ID、类型、名称及连接参数；先接通一种用户实际可用的账号，再补齐另一种。 | P1/P2 |
| D02 | ChatGPT 登录、继续、失败和取消 | 通过当前 app-server 登录协议打开浏览器，展示实际等待/成功/失败状态；不自行实现 OAuth 服务。 | P2 |
| D03 | API provider、URL、模型与密钥 | 表单生成 agentor 自有 Codex 配置；使用当前协议的 provider/认证字段，普通查询不回传密钥。 | P1/P2 |
| D04 | 账号改名与删除 | 改本地账号元数据；当前使用账号先切换后删除；不随之删除会话库。 | P2 |
| D05 | 账号切换 | 活动任务结束后切换，重启连接并加载新模型选项；失败显示原因，保留可重新选择的账号。 | P2 |
| D06 | 每账号模型、effort 与权限偏好 | 保存有效选项；不兼容历史值，不显示未实现的 fast/speed 模式。 | P1/P2 |
| D07 | 连接检查 | 本地校验字段；用户点击验证时做一次必要的线上请求，明确区分配置错误和服务错误。 | P2 |
| D08 | ChatGPT 配额与重置时间 | 展示上游真实 rate limit 数据；与 API 平台余额分开展示。 | P2 |
| D09 | API 余额查询 | Go HTTP + GET/POST 配置、密钥引用、JSON Pointer 字段映射，读取余额/已用/总额/单位。 | P2 |
| D10 | 余额刷新与错误 | 提供手动刷新和短时缓存，HTTP 有超时；结果未知就显示未知，失败不影响聊天。 | P2 |

余额映射只支持 JSON 值提取和明确的数值比例换算，不执行表达式。密钥只在 Go 中填充到请求，不保存到任务参数或前端状态。旧任意 JS 脚本功能不保留。

### E. 工作区

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| E01 | 项目创建、编辑和移除 | 保存名称、实际目录与上下文；移除工作区记录不删除项目目录。 | P3 |
| E02 | 项目共享上下文、提示词和结构说明 | Textarea 编辑，正式派发时读取；结构说明由用户/Agent维护，不添加自动仓库扫描器。 | P3/P4 |
| E03 | 版本创建、编辑和删除 | 项目内 Version 实体，保存名称、说明、顺序。 | P3 |
| E04 | 版本标签 | 保存名称、颜色、顺序，使用 Badge 展示；不引入标签规则系统。 | P3 |
| E05 | 功能创建、编辑和详情 | Feature 保存标题、描述、上下文、提示词，详情页展示步骤和知识文档。 | P3 |
| E06 | 步骤创建、编辑和删除 | Step 保存标题、描述、提示词、会话引用；删除前有运行任务时先停止，避免悬空执行。 | P3/P4 |
| E07 | 层级导航与创建后定位 | 区分选中和展开；新步骤创建后进入详情，恢复最近选中项。 | P3 |
| E08 | 项目、标签和功能排序 | 保存顺序；功能可在同项目版本间移动；不扩展跨项目迁移。 | P3 |
| E09 | 步骤会话与状态 | 首次执行时创建线程并绑定；运行态来自 Execution，未绑定/空闲/运行/成功/错误可见。 | P3/P4 |
| E10 | 工作区持久化和删除范围 | 一份业务数据由 Go 管理；删除关联元数据，不静默删除外部文件和 Codex 会话。 | P3 |

原先未实际生效的 skillSettings 不单独做设置页面。技能相关文字可纳入项目/功能提示词；完整技能管理不属于本版。

### F. 知识文档

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| F01 | Markdown 导入 | 选择本地文件并复制到 agentor 管理目录，保留来源信息，不修改原文档。 | P3 |
| F02 | 文档列表、预览与选择绑定 | 文档有稳定 ID，按项目/功能关联；使用表格/列表、Markdown 预览和选择控件。 | P3 |
| F03 | 取消绑定与删除 | 明确区分移除引用和删除托管副本；不删除原始导入文件。 | P3 |
| F04 | 搜索与读取 | Go 对当前授权范围的 Markdown 文档做标题/内容检索，返回匹配摘要和按需正文。 | P3 |
| F05 | Agent 创建、绑定与同步文档 | 提供 prepare/bind/sync 业务工具，由 Go 生成管理路径并完成写入或更新。 | P4 |
| F06 | 当前步骤上下文 | get_step_context 返回项目、功能、步骤及绑定文档索引，范围由宿主根据线程确定。 | P4 |
| F07 | 文档和上下文展示 | 详情可看到绑定文档及实际执行所用上下文，便于判断信息是否生效。 | P3/P4 |

保留 get_step_context、search_knowledge、read_knowledge、prepare_knowledge_document、bind_knowledge_document、sync_knowledge_document 六类能力；不实现 inspect/migrate 旧存储工具。

### G. 控制器与正式步骤执行

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| G01 | 控制器专用会话 | 单独的只读 Codex 会话，处理工作安排；通过宿主工具修改工作定义或派发任务。 | P4 |
| G02 | 控制器请求队列 | 桌面和飞书共用 Request；持久化 FIFO，展示排队/运行/完成；消息 ID 用于去重。 | P4 |
| G03 | 项目/版本/功能/步骤查询 | 工具返回必要摘要和稳定 ID，详情按需读取。 | P4 |
| G04 | 控制器创建功能和步骤 | 调用 workspace 的既有方法，不另建一套存储。 | P4 |
| G05 | 正式派发 Execution | 生成执行记录，保存对象 ID、指令、完成标准、模型、effort、来源及结果。 | P4 |
| G06 | 上下文注入 | 将项目共享上下文/提示词、功能上下文/提示词、步骤描述/提示词和知识索引组合成执行指令。 | P4 |
| G07 | 桌面与控制器共用步骤入口 | 两者调用同一 dispatch；首次建线程，后续复用；运行中追加与新排队执行分开。 | P4 |
| G08 | 同步骤串行、不同步骤并行 | 每步骤一个执行队列，活动 execution 与 turn 一一关联；不能仅凭当前选中 UI 判定目标。 | P4 |
| G09 | 状态查询、停止与手动继续 | 查询真实 Execution/Turn，停止使用上游 interrupt；失败或中断由用户决定是否再次执行。 | P4 |
| G10 | 目标消歧与来源路由 | 优先明确 ID/引用；“继续”只匹配当前来源最近明确对象，无法唯一确定时询问，不猜任务。 | P4/P6 |
| G11 | 重建控制器/步骤上下文 | 空闲时创建新线程并更新绑定；保留旧记录，避免自动重放旧指令。 | P4 |
| G12 | 流水线控制和完成回报 | 控制器能列配置、触发运行、修改调度开关；收到结果后生成用户可读汇报。 | P5 |

内部角色至少区分普通聊天、控制器、步骤、流水线管理、流水线配置。只注册当前角色实际需要的工具；宿主根据活动线程确定作用域，不接受模型任意指定另一个项目作为事实依据。

### H. 流水线与定时任务

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| H01 | 项目流水线主页 | 每项目显示任务配置、定时任务和最近运行，使用 Tabs/Table/Card。 | P5 |
| H02 | 管理会话与配置会话 | 分开的只读会话；前者查询/运行已有任务，后者通过工具增删改 JSON 配置。 | P5 |
| H03 | 固定动作目录 | Go 中显式注册少量动作及参数定义，界面和 AI 都能读取目录；不扫描动态插件。 | P5 |
| H04 | JSON 任务编辑与验证 | 支持表单和 JSON 文本查看；保存时检查动作、参数和引用，不搭建通用 Schema 引擎。 | P5 |
| H05 | 手动执行与状态 | 顺序执行动作，记录每步输入摘要、输出和错误；失败即停止，可手动再次运行。 | P5 |
| H06 | 一次性定时任务 | 存绝对时间和时区，到期排队一次并停用。 | P5 |
| H07 | 固定间隔、启停和错过触发 | 基于锚点和间隔计算；支持 skip/fire_once，不补跑大量历史周期。 | P5 |
| H08 | 同任务不重入与停止 | 已有活动运行时不并发启动同一任务；context 取消传递到动作及其步骤执行。 | P5 |
| H09 | 结果记录与控制器回报 | 保存运行和动作结果，生成报告产物，结果回到发起请求；飞书发送走通知流程。 | P5/P6 |

流水线只在 agentor 运行时调度，不安装系统守护进程或云端调度服务。

### I. 飞书接入

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| I01 | 应用配置与开关 | App ID、App Secret、飞书/Lark 区域、启用状态；密钥不回显。 | P6 |
| I02 | 长连接和连接状态 | 官方 Go SDK 处理连接；展示未配置、连接中、已连接、错误及必要权限缺口。 | P6 |
| I03 | 联系人/群授权 | 保存允许的发送者和群；候选发现只保存选择所需身份信息。 | P6 |
| I04 | 群内 @ 机器人识别 | 使用 bot 的真实身份判断，群消息仅在允许的群且明确提及时进入业务。 | P6 |
| I05 | 文本、富文本、图片和文件 | 按实际顺序转换为 Request 正文与附件引用，授权通过后下载附件。 | P6 |
| I06 | 来源、引用和去重 | 保存 app/chat/sender/message ID；引用消息用于定位任务，同一消息只入队一次。 | P6 |
| I07 | 远程控制器与结果回复 | 使用同一个控制器队列；结果发回原始会话/消息，不依赖桌面当前页面。 | P6 |
| I08 | 忙碌与完成表态 | 请求处理中可添加表态，完成后更新；表态失败不改变业务执行结果。 | P6 |
| I09 | 审批卡片和文本响应 | 卡片回调直接进入审批处理，不排在待批准任务后面；保留明确审批 ID 的文本入口。 | P6 |
| I10 | 桌面任务审批接收者绑定 | 设置接收人/会话；需要远程确认时使用一次性配对码建立绑定，不自动给群广播。 | P6 |
| I11 | 断线与失败诊断 | 复用 SDK 的连接处理，展示错误和重连；未确认送达的消息不自行重复发送。 | P6 |

飞书官方 Go SDK 提供 API 客户端、WebSocket 及事件/卡片处理能力，接入可参考[官方仓库](https://github.com/larksuite/oapi-sdk-go)和[官方事件示例](https://github.com/larksuite/oapi-sdk-go/blob/v3_main/sample/event/event.go)。P6 开始时选定并锁定一个支持所需 API 的 SDK 版本，不引入旧 CLI。

### J. 附件输入与产物分享

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| J01 | 输入附件登记 | Go 记录附件 ID、来源、文件名、类型、状态、所属请求/步骤和实际管理路径。 | P2/P6 |
| J02 | 飞书媒体下载 | 根据消息资源 ID 下载至管理目录；失败单独展示，不伪造为已成功输入模型。 | P6 |
| J03 | 模型输入适配 | 图片按当前 Codex 输入协议提交；普通文件以当前工具可访问的本地附件信息提供。 | P2/P6 |
| J04 | 文档类附件 | 保存 txt/md/csv/PDF/Office 原文件，展示文件信息；不增加 OCR、Office 转换或宏执行。 | P6 |
| J05 | 产物注册 | Agent 提交当前任务的产物路径，由宿主登记；任务 ID 与执行记录关联。 | P4/P6 |
| J06 | 产物本地查看和分享 | 本地打开/定位，远程通过明确分享工具进入上传和通知流程。 | P6 |
| J07 | 附件生命周期 | 草稿、请求和产物引用清楚；清理只涉及无引用的应用缓存，不按文件名猜归属。 | P6 |
| J08 | 上传结果与逐项状态 | 每个文件独立记录 sent/failed/unknown；部分失败可以针对失败项重试。 | P6 |

文件大小、可访问路径和内容类型只做实际读写所需检查；不复制旧附件的哈希迁移、复杂配额层或历史修复逻辑。

### K. 权限、审批与用户问答

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| K01 | 账号/项目/会话权限配置 | 显示实际支持的权限模式与继承结果，执行前将最终值传给 Codex。 | P2/P4 |
| K02 | 控制器/流水线只读角色 | 由创建线程的权限设置和工具范围共同约束；步骤按用户选择的执行权限运行。 | P4/P5 |
| K03 | 命令、文件和权限审批 | 只对上游真实 server request 建立审批项，展示来源、内容和允许/拒绝操作。 | P2 |
| K04 | 桌面审批中心 | Sheet 展示 pending 与历史，请求切换会话后仍可找到；回复后等待上游状态更新。 | P2 |
| K05 | 结构化问题与 MCP elicitation | Questionnaire 或表单按当前请求字段渲染；答案回到原 request，不当作普通聊天文本发送。 | P2 |
| K06 | 飞书审批归属 | 校验审批 ID、目标接收者和来源会话；一次处理后其他入口显示已处理。 | P6 |
| K07 | 到期、连接失效和重复点击 | 请求到期或运行结束后不可提交；关闭/重连使旧请求失效；同一请求只回应一次。 | P2/P6 |
| K08 | 批准与执行结果分离 | 用户允许只表示回复审批，最终成功/失败仍等待上游执行事件。 | P2 |

不为普通 UI 操作伪造 Codex 审批，也不为每个后端方法另加一轮确认。原权限默认值不是不可更改的迁移要求；首次设置展示明确选项，由用户选定。

### L. 通知与可见运行反馈

| 编号 | 功能点 | agentor 实现思路 | 阶段 |
| --- | --- | --- | --- |
| L01 | 通知中心 | 显示步骤/控制器/流水线结果及发送状态，支持跳转来源对象。 | P4/P6 |
| L02 | 结果和送达分离 | Execution completed 后生成通知，通知失败不把执行改成失败。 | P4/P6 |
| L03 | 文本和产物分项发送 | 一次结果可含多个发送项，分别保存接收目标和状态。 | P6 |
| L04 | 失败重试与忽略 | 重试确定失败的项；状态不明由用户判断；忽略保存原因。 | P6 |
| L05 | 排队、运行与错误提示 | 任务页显示真实阶段，Toast 只作即时反馈，错误详情可在业务页面查看。 | P1/P4/P6 |
| L06 | 重启后记录仍可查看 | 保留新应用自己的任务、通知历史，按简单启动规则恢复显示，不做旧数据导入。 | P4/P6 |

## 6. 固定 Go 动作与 JSON 任务的具体约定

### 6.1 首版动作范围

先实现下列四种动作，覆盖“读取工作状态、执行步骤、读取知识、生成结果报告”的现有工作流程。没有实际使用场景时不添加文件批量操作、任意 HTTP、Shell 或系统管理动作。

| 动作 | 输入 | 输出 | 实现责任 |
| --- | --- | --- | --- |
| `workspace.snapshot` | 当前项目，以及可选的版本/功能 ID | 项目结构、步骤状态摘要 | 调用 workspace/tasks 的既有查询。 |
| `step.run` | 当前项目下的 stepId、instruction、可选完成标准 | executionId、最终状态、结果摘要、产物 ID | 调用正式步骤队列，并等待该 Execution 结束；不另写执行器。 |
| `knowledge.read` | 当前项目/功能内的 documentId | 文档标题和正文 | 调用已有知识文档读取。 |
| `report.write` | 文件名、文字或前序动作结果 | Markdown 报告的 artifactId | 将结果确定性地格式化为 Markdown，写入当前运行的产物目录并登记。 |

`step.run` 等待期间允许真实审批、追加指令和取消。流水线停止时，只取消由本次运行发起的步骤执行，不停止同项目的其他任务。

动作函数由 Go 源码维护，参数类型直接用 Go 结构表示。动作目录向配置会话返回名称、用途、必填参数和结果说明；不要为了四种动作建立插件加载器或完整 JSON Schema 解释器。

### 6.2 任务配置

任务由稳定 ID、名称、输入字段和顺序动作组成。首版只支持：

- 字符串、数字、布尔值等直接参数。
- `{"input":"字段名"}`：引用本次任务的输入字段。
- `{"result":"动作ID"}`：引用已完成动作的完整结果。

引用只发生在整个参数值的位置，不做字符串插值、任意路径求值、条件表达式、循环、动态代码或前向引用。保存时检查引用存在且类型符合动作所需参数。

以下是设计示例，示例中的步骤 ID 是运行时输入，不是待导入的旧数据：

```json
{
  "id": "run-step-and-report",
  "name": "执行步骤并生成报告",
  "inputs": [
    { "name": "stepId", "type": "string", "required": true },
    { "name": "instruction", "type": "string", "required": true }
  ],
  "actions": [
    {
      "id": "execute",
      "type": "step.run",
      "params": {
        "stepId": { "input": "stepId" },
        "instruction": { "input": "instruction" }
      }
    },
    {
      "id": "report",
      "type": "report.write",
      "params": {
        "fileName": "result.md",
        "content": { "result": "execute" }
      }
    }
  ]
}
```

项目归属由宿主的 PipelineRun 注入，不允许引用另一个项目的步骤来改变作用域。普通配置保存只验证，不执行动作。

### 6.3 会话和调度

| 入口 | 可以做什么 | 不能做什么 |
| --- | --- | --- |
| 管理会话 | 列任务/动作、读取契约、调用已有任务、查看运行结果 | 修改动作源码，写入任意本地程序。 |
| 配置会话 | 新建/修改/删除 JSON 任务，维护定时配置，调用只读验证 | 安装依赖、生成可执行脚本、变更 Go 动作实现。 |
| 控制器 | 查询任务、触发管理/配置请求、启停或触发调度、汇报结果 | 绕过动作和步骤范围直接执行任意程序。 |
| 调度器 | 在应用运行时按既定输入触发任务 | 按时自动生成新代码、创建系统后台服务。 |

每个项目保留独立的管理会话和配置会话。针对同一会话的请求串行；不同项目互不阻塞。创建 PipelineRun 时固定任务配置快照和输入，运行中编辑不会改变这次执行。

一次性与固定间隔调度保存 `enabled`、时间/间隔、时区、输入和 `misfirePolicy`。时间内部统一存为 UTC，时区用于输入和展示。启用 Go 时区数据后即可避免依赖特定机器的时区文件布局。

同一任务已有 queued/running 时，本次定时触发记为跳过，避免无限堆积。动作失败不自动重试整个任务；用户可从运行记录再次执行。

## 7. 动态工具与服务边界

不按旧工具的数量机械复制；按以下业务能力注册。具体名称在 P4/P5 中一次确定，前端和工具调用复用相同的业务方法。

| 使用者 | 宿主提供的能力 | 主要实现位置 |
| --- | --- | --- |
| 普通聊天 | 当前 Codex 原生能力、真实审批与问答 | codex |
| 步骤会话 | 读取步骤上下文、搜索/读取知识、准备/绑定/同步知识、登记/分享产物 | workspace + artifact + tasks |
| 控制器会话 | 查工作区、建功能/步骤、派发/停止/查执行、请求重建空闲上下文、操作流水线 | workspace + tasks + pipeline |
| 流水线管理会话 | 列动作/任务、执行任务、读取结果 | pipeline |
| 流水线配置会话 | 保存/删除任务、保存/删除调度、验证配置 | pipeline |
| 飞书 | 提交已有 Request、回答真实审批、发送已生成结果 | lark 调用既有业务入口 |

普通前端方法使用明确的参数和结果类型；工具适配处只做参数解码、调用和结果编码。不得分别为桌面、控制器、飞书实现三份派发逻辑。

重要事件至少包含所属 threadId/turnId 或 requestId/executionId，按实际类型携带。事件只推送 UI 更新所需数据，详细记录按需查询。停止任务不能靠停止前端订阅实现。

## 8. 开发流与预计周期

### 8.1 估算前提

按一名主要开发者配合 AI、工作日持续投入、所需 Codex 账号和飞书测试配置可用估算：

- 核心实施约 **27–35 个工作日**。
- 给 Wails WebView 差异、当前 Codex 协议和飞书联调留 **3–5 个工作日**。
- 整体按 **30–40 个工作日，约 6–8 周** 安排。

这是工作量估算，不是固定日历承诺。若只在零散时间开发，应按实际投入延长；不通过删减功能来宣称按期完成。P1 完成真实协议接通后再校准剩余估算。

### 8.2 三条开发流

| 开发流 | 责任 | 配合方式 |
| --- | --- | --- |
| UI 与交互 | shadcn 选择、页面组合、前端状态、事件显示 | 每阶段跟随真实后端完成一条流程，允许暂用样例数据排版，但交付前换成真实调用。 |
| Go 与 Codex 核心 | 生命周期、协议、账号、存储、会话与审批 | 先建立真实会话，再供工作区和控制器复用。 |
| 工作编排与集成 | 工作区、知识、执行、流水线、飞书和通知 | 按依赖接入已有入口，避免另造运行时。 |

这里的“开发流”表示职责划分，不要求同时启动多个 Agent。单人可在同一阶段内交替推进。若未来明确采用多人/多 Agent，应分配到 feature/Go 包，`main.go`、service 方法、共享模型和 bindings 由一个负责人协调。

### 8.3 里程碑总表

| 阶段 | 时间 | 前置 | 主要功能 | 可交付结果 |
| --- | --- | --- | --- | --- |
| P0 工程与外壳 | 1 天 | 无 | A01–A04、A06、A08 的基础部分 | Wails 窗口可运行、主题和导航可用、Go 调用能返回真实结果。 |
| P1 真实 Codex 对话 | 4–5 天 | P0 | A05/A07、B01–B07/B10、C01–C03/C05、D01/D03/D06 的基础部分 | 通过一种新配置的可用账号完成真实对话、追加、停止和重开历史。 |
| P2 完整聊天体验与账号/审批 | 4–5 天 | P1 | B08/B09、C04–C15、D02–D10、K01/K03–K05/K07/K08、J01/J03 的本地部分 | 账号切换、富消息、图片、工具过程、余额和真实桌面审批可用。 |
| P3 工作区与知识 | 4–5 天 | P1，可与 P2 的独立 UI 工作交错 | E01–E10 的定义与展示、F01–F04/F07 | 项目到步骤的完整管理、文档导入绑定与持久化可用。 |
| P4 控制器与执行闭环 | 4–5 天 | P2、P3 | E02/E06/E09、F05–F07、G01–G11、K01/K02、J05、L01/L02/L05/L06 | 桌面或控制器派发同一套步骤执行，能排队、停止、查看结果。 |
| P5 Go 流水线与调度 | 3–4 天 | P4 | G12、H01–H09、K02 | 配置会话生成 JSON，固定 Go 动作运行，定时触发并回报。 |
| P6 飞书、附件与通知 | 5–7 天 | P4；流水线联动依赖 P5 | I01–I11、J01–J08 的完整链路、K06/K07、L01–L06 | 飞书提交任务、审批、上传产物、结果回原会话和失败重试。 |
| P7 当前平台运行验收 | 2–3 天 | P0–P6 | 第 9 节场景、实际缺陷修正 | 当前机器完整流程可运行，记录通过项和外部条件阻塞项。 |

### 8.4 每阶段执行顺序与完成条件

**P0：先拿到可运行窗口。**

1. 阅读 demo 的入口、service、前端别名/主题、组件配置及开发脚本。
2. 初始化 agentor 自己的工程信息，仅引入需要的基础依赖。
3. 完成窗口、侧栏、主题、空状态、设置页和数据目录。
4. 从 Wails 界面调用一个实际 Go 方法；关闭后再启动，确认设置保存。

主要文件：`main.go`、开发配置、`internal/appdata/`、`frontend/src/app/` 和基础 shadcn 组件。完成条件：用 Wails dev 启动真实窗口，页面没有持续运行时错误。

**P1：先走通真实协议，避免后续全部依赖模拟数据。**

1. 在用户选择的当前原生 Codex 版本上确认 initialize、会话、模型、turn 的实际请求和事件。
2. 建立独立 `CODEX_HOME`，完成一种账号和连接配置；有 API 账号时优先使用，否则将 D02 的最小 ChatGPT 登录通路提前到本阶段。
3. 实现会话列表、新建、恢复、发送、追加、停止和流式文字。
4. 实现目录分组、标题/归档、基础模型选项和线程事件归属。

主要文件：`internal/codex/`、`features/chat/`、`features/accounts/`。完成条件：一轮真实任务完成；切换会话不串消息；停止有效；重开能读到本应用的新历史。如果实际 Codex 协议与源码不同，在这一阶段调整当前版本适配，不增加旧协议兼容层。

**P2：补齐完整聊天与上游交互。**

1. 依次接入 Trace、代码、diff、图片、历史分页、滚动与上下文使用量。
2. 接入真实审批、结构化问答、ChatGPT 登录及账号切换。
3. 补齐配额、余额查询、worktree、HTML 可视化。
4. 提前验证图片粘贴和 iframe 隔离在当前 WebView 的表现。

主要文件：`internal/codex/`、`internal/artifact/`、`features/chat/`、`features/accounts/`、`features/approvals/`。完成条件：常用消息类型可见，图片可输入，至少一个真实审批能闭环，账号切换不混用任务和配置。

**P3：建立工作数据，不提前制造执行系统。**

1. 完成项目、版本、功能、步骤的定义、编辑、导航和排序。
2. 完成共享上下文/提示词和文档导入、绑定、预览。
3. 确认重启后层级、顺序、标签、文档引用还在。
4. 为 P4 提供查询当前步骤完整上下文的方法。

主要文件：`internal/workspace/`、`features/workspace/`、`features/knowledge/`。完成条件：创建一套项目→版本→功能→步骤，绑定 Markdown 后可重新打开，无旧数据依赖。

**P4：将桌面步骤与控制器接到同一执行入口。**

1. 建立 Request/Execution，先完成单步骤正式执行。
2. 补齐上下文注入、知识工具和产物登记。
3. 再接控制器查询、创建、派发、排队、停止与上下文重建。
4. 完成基本通知记录和重启中断展示。

主要文件：`internal/tasks/`、`tools.go`、`features/controller/`，以及 workspace/artifact 的必要接口。完成条件：同一步骤顺序执行，不同步骤可并行；控制器能创建并派发；运行结果回到正确来源。

**P5：流水线只编排已经存在的能力。**

1. 实现四个固定动作和顺序执行器。
2. 完成 JSON 配置保存、引用解析、动作结果和报告。
3. 接入独立管理/配置会话及控制器入口。
4. 实现一次性/间隔调度、启停、错过策略和不重入。

主要文件：`internal/pipeline/`、`features/pipeline/`、`tools.go`。完成条件：第 6 节示例流程实际执行并产出报告；短间隔定时任务可触发和停用，全程不启动脚本解释器。

**P6：飞书只接入已经跑通的业务流程。**

1. 先接 Go SDK 长连接、身份和授权范围。
2. 走通文本任务→控制器→结果回复。
3. 接真实审批卡片/文本响应，再接输入附件和产物上传。
4. 补齐表态、通知分项状态、失败重试和配置诊断。

主要文件：`internal/lark/`、`features/lark/`、`features/notifications/`，以及 artifact/tasks 的发送记录。完成条件：在用户指定的测试会话完成一轮真实往返，回复和审批始终归属原消息。

**P7：只修复阻止使用的问题。**

1. 在当前平台运行第 9 节的小型场景集。
2. 修正阻塞主流程的问题，确认退出和重开可用。
3. 给出启动方式、已验证能力和未满足的外部条件。

不在此阶段补做架构重构、主题重设计、旧数据兼容或发布构建。

### 8.5 每轮编码的交付粒度

- 每轮完成一个可操作的小流程，例如“新建并打开步骤”，不单独交付一批没有调用方的接口。
- UI 选择与后端方法在同一轮接起来，涉及 Go 方法变化时重新生成 bindings。
- 正常开发中的类型检查、格式化和必要编译随改动执行，不作为额外大型验收阶段。
- 仅对实际存在的复杂逻辑补少量针对性检查，例如同一步骤排队、审批只回应一次、调度不重入；不为简单字段保存编写镜像测试。
- 每阶段结束更新第 11 节进度，用简短记录写清“完成了什么、如何运行验证、剩余什么”。不会因为阶段结束就重跑全部检查。

## 9. 简单验收

### 9.1 验收环境

当前工作机是 macOS。最终检查在 **Wails 桌面窗口**中完成，不能以浏览器中的 Vite 页面或模拟 API 通过代替桌面联调通过。

开发启动沿用 demo 风格，在 agentor 根目录使用 `./scripts/wails.sh dev`。具体命令在 P0 实际建立并记录。Wails dev 所需的当前平台编译和前端资源处理属于运行前准备；不执行跨平台发布构建或打包矩阵。

新建测试项目、测试会话和应用数据；不复制旧应用数据。飞书检查使用用户明确允许发送的测试会话，缺少账号或权限时写明未验证项，不能将模拟结果标成真实通过。

### 9.2 最终只保留这 9 组场景

| 编号 | 操作 | 通过标准 |
| --- | --- | --- |
| V1 启动与保存 | 启动、切换主题、选项目目录、退出再启动 | 窗口正常、设置保存、后台连接可正确关闭重建。 |
| V2 基础对话 | 新建会话、发送、追加、停止、切换两个会话、重开历史 | 事件归属正确，停止有效，历史来自本应用。 |
| V3 富消息 | 图片选择/粘贴、Markdown/代码/Mermaid、工具输出/diff、长消息和 HTML 示例 | 可读可操作，滚动不抢用户位置，预览不获得宿主绑定。 |
| V4 账号与审批 | 新增/切换账号，查看一次配额或余额，完成一次真实审批/问答 | 账号配置生效、数值来源真实、答案回原 request。具备两类账号时分别验证。 |
| V5 工作区与知识 | 建项目→版本→功能→步骤，改顺序/标签，导入并绑定 Markdown，重启 | 数据和关联保存，正式执行能拿到对应上下文。 |
| V6 控制器与队列 | 控制器派发步骤，同一步骤再排一项，另一条步骤并行，停止一项 | 顺序、并行和停止正确；不影响其他任务；结果可定位来源。 |
| V7 流水线 | 配置会话保存 JSON，手动执行；设短间隔任务后停用 | 固定 Go 动作执行并产出报告，定时任务不重入，停用后不再触发。 |
| V8 飞书闭环 | 在允许的测试会话发任务及附件，完成审批，收到文字和产物 | 来源正确、重复事件不重复执行，上传/发送结果可见，失败项可重试。 |
| V9 简单异常与重开 | 断开一次连接或取消任务；带运行记录退出重开 | 错误原因可见；原运行任务显示中断，不自动重放；不需要清空数据才能再次使用。 |

验收记录只需“通过/未通过/外部条件阻塞 + 一句证据”。无需截图全集、各平台安装包、覆盖率指标或压力测试报告。没有环境验证的平台写“未验证”，不要因使用 Wails 就写成三平台全部通过。

## 10. 参考代码与阅读顺序

### 10.1 先读 demo 的工程方式

| 参考 | 需要学习的内容 |
| --- | --- |
| [main.go](/Users/zhui/Work/p/wails/demo/main.go) | Wails 应用装配、资源嵌入、service 注册、窗口。 |
| [service.go](/Users/zhui/Work/p/wails/demo/internal/demo/service.go) | 明确职责的 Go 方法和生成绑定。 |
| [official-practices.md](/Users/zhui/Work/p/wails/demo/docs/official-practices.md) | 官方模板边界、版本和开发约定。 |
| [components.json](/Users/zhui/Work/p/wails/demo/frontend/components.json) | shadcn Base UI 风格与路径别名。 |
| [catalog.ts](/Users/zhui/Work/p/wails/demo/frontend/src/features/showcase/catalog.ts) | 已有组件目录，尤其 conversation 组。 |
| [ui 组件目录](/Users/zhui/Work/p/wails/demo/frontend/src/components/ui) | 优先复用的组件实现。 |
| [wails.sh](/Users/zhui/Work/p/wails/demo/scripts/wails.sh) | 本机 Go SDK、Wails CLI 和开发入口。 |

### 10.2 再按当前阶段查 CodexPFWork 的业务

| 业务 | 源码入口 | 阅读重点 |
| --- | --- | --- |
| 协议 | [app-server-client.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/app-server-client.js) | 初始化、请求/通知/server request；只参考当前业务语义。 |
| 界面行为 | [renderer/app.js](/Users/zhui/Work/p/wails/CodexPFWork/src/renderer/app.js) | 交互、消息分类、工作区导航；不移植巨型 DOM 操作文件。 |
| 应用调度 | [main.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/main.js) | 找实际调用链；忽略测试捕获和 Electron 专属逻辑。 |
| 账号 | [account-profile-store.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/account-profile-store.js)、[codex-auth-config.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/codex-auth-config.js) | 当前账号行为与认证配置；跳过迁移分支。 |
| 余额 | [api-balance-service.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/api-balance-service.js) | 请求/结果语义；不移植 JS 沙箱。 |
| 工作区 | [pfwork-store.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/pfwork-store.js) | 项目/版本/功能/步骤关系。 |
| 知识 | [pfwork-managed-knowledge.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/pfwork-managed-knowledge.js)、[pfwork-knowledge-tools.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/pfwork-knowledge-tools.js) | 当前文档导入、绑定和工具语义。 |
| 控制器 | [pfwork-controller-service.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/pfwork-controller-service.js)、[pfwork-controller-tools.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/pfwork-controller-tools.js) | 请求来源、队列、dispatch 和完成结果。 |
| 流水线 | [pfwork-pipeline-service.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/pfwork-pipeline-service.js)、[shared/pfwork-pipeline.js](/Users/zhui/Work/p/wails/CodexPFWork/src/shared/pfwork-pipeline.js) | 双会话、调度和运行记录；动作实现按本计划重新设计。 |
| 飞书 | [lark-integration-service.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/lark-integration-service.js)、[lark-bot-bridge.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/lark-bot-bridge.js) | 路由、身份、消息来源；底层换 Go SDK。 |
| 附件 | [attachment-store.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/attachment-store.js)、[pfwork-attachments.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/pfwork-attachments.js) | 归属、模型输入、产物登记和分享。 |
| 审批 | [approval-service.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/approval-service.js)、[approval-protocol.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/approval-protocol.js) | 上游请求的类型、回答及失效。 |
| 通知 | [pfwork-notification-record.js](/Users/zhui/Work/p/wails/CodexPFWork/src/main/pfwork-notification-record.js) | 任务结果与送达记录的区别；不复制迁移/修复体系。 |

参考项目只读。出现“源代码存在，但本计划明确不移植”的内容时，以本计划的新应用范围为准。不要为找一个功能再次完整扫描所有源码和旧测试。

## 11. 编码进度与交接

### 11.1 进度

2026-10-20 用户执行调整：开发与联调期间先复用当前 CLI 的原生 Codex、登录状态和连接配置；整体目标完成后再处理独立账号配置。此调整仅为临时接入方式，不读取或迁移参考应用的数据，不改写 CLI 原有连接配置。其余技术与范围约定继续有效。

2026-10-20 用户补充目标：遇到飞书 bot 密钥、流水线地址等需要用户提供的第三方信息时，可暂跳过真实验证，继续其他能完成的任务；最终明确汇报待验证项，由用户决定后续处理。外部条件不足不构成提前停止全部实现的理由。

2026-10-20 用户补充执行约定：按已验证的实现阶段进行本地 Git commit，保留可回溯的开发记录；提交说明与交接记录应反映真实完成范围。

- [x] 确认源码目录和 demo 参考方式。
- [x] 确认 Node 仅作为开发工具。
- [x] 确认固定 Go 动作 + JSON 流水线方案。
- [x] 落地本计划（制定时未开始编码）。
- [x] P0 工程与外壳。
- [x] P1 真实 Codex 对话（按用户调整临时复用 CLI 登录与连接配置）。
- [ ] P2 完整聊天、账号与审批。
- [ ] P3 工作区与知识。
- [ ] P4 控制器与步骤执行。
- [ ] P5 Go 流水线与调度。
- [ ] P6 飞书、附件与通知。
- [ ] P7 当前平台运行验收。

每个编码会话在此追加简短交接记录，包含已完成阶段/功能编号、真实运行结果、未完成事项和下一步。不要把尚未运行的功能标成完成。

#### 2026-10-20：P0 完成，P1 开始

- P0（A01–A04 基础、A06、A08）：已建立独立 Go module、前端依赖和锁文件、按需 shadcn 组件、生成的 bindings、开发 Taskfile 与入口。桌面可切换主题/工作模式、选择项目目录、保存设置、查询 Go 实际运行信息。
- 真实验证：`./scripts/wails.sh dev` 启动 macOS Wails 窗口；从界面切换深色主题、通过原生对话框选取 `/private/tmp/agentor-project-check`、切换工作区、退出再启动，主题/目录/模式均恢复。测试业务数据位于 `/private/tmp/agentor-desktop-check`。前端类型检查与开发编译通过。
- 开发环境说明：沙盒会阻止 macOS FSEvents，桌面开发验证需允许原生文件监听。开发 app 内可执行文件使用新文件后替换，避免覆盖原 inode 后重开被 macOS 终止。不制作发布安装包。
- P1 已接通 stdio JSON-RPC 的 initialize/initialized、原生程序定位、设置页连接状态和重连。真实窗口显示当前 CLI `0.162.1` 的原生路径与共享 `CODEX_HOME`。身份查询返回上游 `workspace routing discovery timed out`，尚未完成真实对话验收；继续接入模型和会话流程并排查。JSON-RPC 并发响应归属、服务器请求区分、断开后结束等待请求的针对性 race 测试通过。
- 后续继续 P1 会话、发送/追加/停止及历史；其余阶段尚未完成。临时沿用 CLI 配置的用户调整见 11.1，整体目标完成后再处理隔离配置。

#### 2026-10-20：P1 真实对话通过

- P1 已接入原生程序定位、initialize/initialized、运行状态与重连、当前身份、真实模型和 effort、会话创建/恢复/目录分组/分页折叠/改标题/归档、项目展示名与移除分组、Markdown 基础呈现、逐会话草稿、发送/追加/停止和历史分页。按用户调整暂用 CLI 身份，独立账号配置留待后续阶段。
- 当前 CLI `0.162.1` 真实会话来源实际为 `vscode`；已纠正过窄的来源筛选。共享 CLI 配置时按本应用记录的会话 ID 与目录筛选列表；不展示其他 CLI 会话。子进程不继承宿主 Agent 的线程 ID、originator 覆盖、沙盒标识和工具管道，仍按用户授权沿用 CLI 认证与连接配置。
- 真实桌面证据：在 `/private/tmp/agentor-project-check` 的会话中收到“agentor 连接验证成功。”；执行 `sleep 60` 后成功追加指令；另建第二会话收到 `SECOND_THREAD_OK`；切回第一会话停止，显示中断，第二会话结果保留且未串消息。界面模型列表来自 app-server，选中 `GPT-6.1-Sol` 后显示支持的 `medium` effort 并保存。
- 退出桌面释放会话写入方后，原生 app-server 的真实 `thread/read/resume/turns/list` 检查通过；桌面重开亦显示两轮用户与助手历史。测试数据目录仍为 `/private/tmp/agentor-desktop-check`。早前身份查询超时未在后续连接中再现。
- 已提前接入 P2 的命令/文件审批与结构化问答入口；重复回应、单轮结束只使对应审批失效、协议并发响应归属、断开结束等待、思考历史解码及宿主环境隔离的针对性 race 测试通过。真实审批、权限审批和 MCP elicitation 尚需继续完成/验证。
- 前端类型检查、开发编译及当前 macOS Wails 窗口启动通过；未开展跨平台构建。下一步 P2，P3 可按计划与独立界面工作交错。

#### 2026-10-20：P2 第一条图片与富消息流程

- 已实现 Go 附件管理、原生文件选择、WebView 图片粘贴、缩略图/原图预览、Wails 资源中间件、按会话与用户消息归属的附件记录，以及当前 Codex `localImage` 输入。普通文件通过当前协议的 `additionalContext` 提供本地文件信息，正文与附件分开展示。
- 真实桌面验证：选择 PNG 后缩略图和原图预览正确；用系统“预览”复制图片，粘贴至 agentor 后生成独立 PNG 附件；发送后模型正确回复“蓝色”。当前 WebView 的图片 paste 事件可用，没有增加平台专用剪贴板代码。
- 已组合 shadcn CodeBlock、Trace、Tabs 和现有 Markdown 组件，接入代码复制、消息原文/复制、大消息展开和按需加载的 Mermaid 12.2.0；Mermaid 仅在轮次完成后解析，失败可查看源码。Mermaid 的真实图表显示、其余富工具视图与 HTML 预览仍待验证/完成。
- 流式 UI 更新按动画帧批量合并，按 item ID/client message ID 去重；缓存保留最近 8 个空闲会话及活动会话，滚动复用 MessageScroller。真实图片验证发现上游部分完成快照会覆盖用户消息，已改为合并条目；3 项针对性前端状态测试通过，历史中的用户消息及图片已重新在桌面确认。
- 应用启动自动建立 Codex 连接，退出按生命周期关闭原生进程；状态按连接代次与修订号接收，旧进程的状态不能覆盖新连接。Go 的附件归属/资源路径检查、连接状态和既有协议/审批 race 测试通过；前端类型检查与开发构建通过。
- 已按用户要求初始化本地 Git 仓库，首次阶段提交包含 P0/P1 与上述 P2 基础。P2 仍未全部完成；继续完善 diff、用量、HTML、账号/余额、权限审批与问答、worktree 和排序。

### 11.2 可直接发给新会话的指令

> 请执行 `/Users/zhui/Work/p/wails/agentor/PLAN.md` 中的开发计划。
>
> 项目目录是 `/Users/zhui/Work/p/wails/agentor`，参考架构是 `/Users/zhui/Work/p/wails/demo`，业务参考源码是 `/Users/zhui/Work/p/wails/CodexPFWork`，不要读取“副本”作为主参考。先阅读计划和当前进度，再从第一个未完成阶段开始编码。
>
> 按全新应用实现，不迁移旧账号、旧文件、旧数据，也不保留 Electron、Node 运行器和旧兼容分支。Node 仅可作为 npm/Vite/shadcn 开发工具。流水线采用固定 Go 动作与 JSON 配置，AI 只配置已有动作。
>
> 所有 UI 先检查 demo 和 shadcn 官方是否有直接可用的组件，优先复用和组合。Go 后端保持简单、可读，不做推测性的抽象和防护体系。桌面、控制器、流水线和飞书共用已有业务入口。
>
> 按里程碑完成真实可运行的流程，并更新 PLAN.md 的进度。最终只检查当前平台的 Wails 运行时，使用计划中的简单验收，不构建各平台发布文件。缺少真实账号或飞书测试条件时明确记录，不用模拟结果冒充通过。demo 和 CodexPFWork 保持只读。

### 11.3 开始编码前还需从实际环境取得的信息

这些是运行时配置，不影响本计划定稿，也不需要现在反复确认：

- 用户准备使用的原生 Codex 可执行文件及其版本。
- 一个新配置的可用账号；ChatGPT 登录和 API 余额验证分别需要对应账号条件。
- 用户允许操作的临时项目目录，以及 Git worktree 检查所用的测试仓库。
- 飞书测试应用、已开通的事件/权限、允许发送的测试会话和审批接收者。

先实现不依赖上述外部条件的工作；进行真实接入时再取得所需配置。不要读取或复制旧应用的密钥来跳过新应用配置。



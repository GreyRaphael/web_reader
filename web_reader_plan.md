# Web Reader 开发计划

> 文档版本：v1.0  
> 制定日期：2026-07-20  
> 项目目录：`/home/gewei/workspace/web_reader`

## 1. 项目目标

实现一个部署在远程 Linux 服务器上的只读 Web Reader：

- 服务默认监听 `0.0.0.0:8848`，用户通过 `http://remote_ip:8848` 访问。
- 当前仅支持一个 `admin` 用户，通过用户名、密码登录。
- 服务启动时绑定一个固定的 `workspace`，用户可以浏览其下所有目录和文件。
- 桌面端采用“文件树 + 内容预览 + Markdown 大纲”的三栏布局。
- 移动端优先，左右栏在小屏幕上切换为可触摸操作的抽屉，阅读区保持最大可用空间。
- 支持 Markdown、普通文本和图片预览；其他文件至少可查看元信息并下载。
- Markdown 支持相对图片、公式、代码高亮、Mermaid 和日间/夜间主题。
- 第一阶段保持只读，不提供上传、编辑、删除和重命名能力。

## 2. 需求范围与验收映射

| 编号 | 需求 | 计划实现 | 验收标准 |
| --- | --- | --- | --- |
| R1 | 远程服务器提供 8848 端口服务 | Go HTTP 服务默认绑定 `0.0.0.0:8848`，可通过参数或环境变量覆盖 | 远程 PC 和手机可打开首页及登录页 |
| R2 | 单 admin 用户登录 | bcrypt 密码哈希、服务端 session、HttpOnly Cookie | 未登录不能访问任何 workspace API；正确凭据可登录和退出 |
| R3 | 访问 workspace 下所有文件 | 懒加载文件树，所有 API 仅接受 workspace 相对路径 | 可展开任意层级；路径穿越和 workspace 外符号链接被拒绝 |
| R4 | 三栏 Web UI | 桌面三栏、可隐藏文件树和大纲、桌面端支持栏宽调整 | 宽屏可同时查看三栏并独立滚动 |
| R5 | 移动端优先 | 左右栏改为抽屉，阅读区单栏，44px 触控目标，适配安全区和动态视口 | 常见手机竖屏下无页面级横向滚动，可单手打开文件与大纲 |
| R6 | Markdown 相对图片 | 根据当前 Markdown 文件目录重写资源地址到受保护的 raw API | `chapter1.md` 中 `./imgs/image1.jpg` 可显示 |
| R7 | 图片查看 | 图片文件使用专用查看器，后端流式传输 | JPG/PNG/GIF/WebP/BMP 等可直接预览 |
| R8 | 公式 | KaTeX，至少支持 `$$...$$`、`\(...\)`，同时兼容 `$...$`、`\[...\]` | 行内和块级公式均能渲染，错误公式不导致整页失败 |
| R9 | 代码块 | `highlight.js` 按语言高亮，未知语言回退纯文本 | fenced code block 正常显示并可横向滚动 |
| R10 | Mermaid | Mermaid 异步渲染，主题联动，单图错误隔离 | 流程图、时序图、类图、状态图、ER、甘特图、饼图、脑图等可渲染 |
| R11 | txt/log/md/image 点击预览 | 按文件类型分发到 Markdown、文本、图片或通用文件查看器 | 示例目录中的 `.md`、`.txt`、`.log`、图片均可查看 |
| R12 | Day/Night 主题 | CSS variables + 本地持久化；默认跟随系统，可手动切换 | 刷新后保留选择；Markdown、代码、KaTeX、Mermaid 同步换肤 |

## 3. 参考项目结论

已参考 `/home/gewei/workspace/md-reader` 的 UI 和渲染思路。该项目使用 Vue 3，并采用以下结构：

- 左侧递归文件树、中间 Markdown 阅读区、右侧大纲。
- `markdown-it` 解析 Markdown。
- `DOMPurify` 清理 HTML。
- `highlight.js` 渲染代码块。
- KaTeX 渲染公式。
- Mermaid 渲染图表。
- 标题锚点、滚动监听和大纲高亮联动。
- CSS variables 管理日间/夜间主题。

Web Reader 将复用这些成熟的交互原则，但不会照搬 Tauri 文件 API、编辑器、导出、桌面窗口等不适用于 Web 服务的功能。参考项目目前主要面向桌面布局，本项目会重新设计响应式行为。

## 4. 总体技术方案

### 4.1 技术选型

#### 后端

- Go 1.22+。
- 优先使用标准库 `net/http`、`http.ServeMux`、`log/slog`，减少运行依赖。
- `golang.org/x/crypto/bcrypt` 保存和校验管理员密码哈希。
- 使用 `embed.FS` 将前端生产构建产物嵌入 Go 二进制。
- 不使用数据库；管理员配置来自环境变量/启动参数，session 保存在进程内存。

#### 前端

- Vue 3 + TypeScript + Vite。
- 使用 Composition API；第一阶段状态规模较小，不强制引入 Pinia。
- `markdown-it`：Markdown 解析。
- `markdown-it-anchor` 或等价自定义规则：稳定且唯一的标题 ID。
- 自定义数学分隔符插件 + KaTeX：明确支持 `$...$`、`$$...$$`、`\(...\)`、`\[...\]`。
- `highlight.js`：代码高亮，按需注册常用语言。
- Mermaid：图表渲染，锁定经过验证的稳定版本。
- `DOMPurify`：渲染 HTML 的二次清理。
- Vitest + Vue Test Utils：前端单元/组件测试。
- Playwright：桌面和移动端端到端测试。

### 4.2 生产形态

前端构建后输出到 Go 可嵌入目录，最终部署一个 Go 可执行文件：

```text
Browser
  │ HTTP :8848
  ▼
Go Web Reader
  ├── /api/*        登录、文件树、文本、文件流
  ├── /healthz      健康检查
  └── /*             嵌入的 Vue SPA 静态资源
        │
        ▼
Configured workspace (read-only)
```

优点：

- 远程服务器不需要 Node.js 运行时。
- 不需要独立部署 Nginx 才能运行基本服务。
- 前后端同源，Cookie、图片和 API 不需要额外 CORS 配置。
- 发布、升级和回滚只涉及单个二进制及配置文件。

## 5. 建议目录结构

```text
web_reader/
├── cmd/
│   └── web-reader/
│       └── main.go                 # 程序入口、信号和优雅退出
├── internal/
│   ├── auth/
│   │   ├── handler.go              # 登录、退出、会话查询
│   │   ├── middleware.go           # 登录保护、Cookie 校验
│   │   ├── session.go              # 内存 session store
│   │   └── limiter.go              # 登录限流
│   ├── config/
│   │   └── config.go               # 参数、环境变量、启动校验
│   ├── filesystem/
│   │   ├── service.go              # 列目录、读文本、文件元信息
│   │   ├── path.go                 # 路径归一化和 workspace 边界检查
│   │   └── type.go                 # 文件类型和预览类型识别
│   ├── server/
│   │   ├── server.go               # 路由和 HTTP server
│   │   ├── middleware.go           # 日志、安全响应头、恢复、请求 ID
│   │   └── spa.go                  # SPA fallback
│   └── webui/
│       ├── embed.go                # `go:embed dist`
│       └── dist/                   # 前端构建输出，生成目录
├── web/
│   ├── src/
│   │   ├── api/                    # typed fetch client
│   │   ├── components/
│   │   │   ├── AppShell.vue
│   │   │   ├── AppToolbar.vue
│   │   │   ├── FileTree.vue
│   │   │   ├── FileTreeNode.vue
│   │   │   ├── SideDrawer.vue
│   │   │   ├── TocPanel.vue
│   │   │   ├── PreviewPane.vue
│   │   │   ├── MarkdownViewer.vue
│   │   │   ├── TextViewer.vue
│   │   │   ├── ImageViewer.vue
│   │   │   └── UnsupportedViewer.vue
│   │   ├── composables/
│   │   │   ├── useAuth.ts
│   │   │   ├── useFileTree.ts
│   │   │   ├── usePreview.ts
│   │   │   ├── useMarkdown.ts
│   │   │   ├── useScrollSpy.ts
│   │   │   └── useTheme.ts
│   │   ├── styles/
│   │   │   ├── tokens.css
│   │   │   ├── layout.css
│   │   │   └── markdown.css
│   │   ├── views/
│   │   │   ├── LoginView.vue
│   │   │   └── ReaderView.vue
│   │   ├── App.vue
│   │   └── main.ts
│   ├── tests/
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── e2e/
├── testdata/
│   └── workspace/                   # 测试 Markdown、文本、图片、Mermaid
├── deploy/
│   ├── web-reader.service           # systemd 示例
│   └── web-reader.env.example
├── .gitignore
├── Makefile
├── go.mod
├── README.md
└── web_reader_plan.md
```

## 6. 后端详细设计

### 6.1 配置

配置优先级：命令行参数 > 环境变量 > 安全默认值。

| 参数 | 环境变量 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--addr` | `WEB_READER_ADDR` | `0.0.0.0:8848` | HTTP 监听地址 |
| `--workspace` | `WEB_READER_WORKSPACE` | 无 | 必填，启动时解析成绝对真实路径 |
| `--admin-user` | `WEB_READER_ADMIN_USERNAME` | `admin` | 当前唯一管理员用户名 |
| `--password-hash` | `WEB_READER_ADMIN_PASSWORD_HASH` | 无 | 必填，bcrypt 哈希，禁止硬编码默认密码 |
| `--session-ttl` | `WEB_READER_SESSION_TTL` | `24h` | 登录会话有效期 |
| `--max-text-size` | `WEB_READER_MAX_TEXT_SIZE` | `10MiB` | Markdown/文本预览上限 |
| `--secure-cookie` | `WEB_READER_SECURE_COOKIE` | `false` | HTTPS 部署时必须开启 |
| `--trust-proxy` | `WEB_READER_TRUST_PROXY` | `false` | 仅在可信反向代理后启用 |

额外提供 `web-reader hash-password` 子命令，从终端安全读取密码并输出 bcrypt 哈希，避免把明文密码写入仓库或 shell history。

启动阶段必须完成：

1. 校验 workspace 存在且是可读目录。
2. 使用 `filepath.Abs` + `filepath.EvalSymlinks` 固定 workspace 的真实根路径。
3. 校验用户名和密码哈希已配置。
4. 打印监听地址、workspace、版本等非敏感信息。
5. 任一关键配置无效时立即退出，不以不安全默认值继续运行。

### 6.2 认证与会话

#### 登录流程

1. 浏览器提交 `POST /api/auth/login`，请求体为 JSON。
2. 服务端检查请求体大小、用户名、来源和登录频率。
3. 使用 bcrypt 校验密码；用户名不存在时也执行等价成本校验，降低时间侧信道差异。
4. 生成至少 32 字节的密码学随机 session token。
5. 仅把 token 的 SHA-256 摘要作为 map key 保存在内存中。
6. 返回 Cookie：`HttpOnly; SameSite=Lax; Path=/`；HTTPS 时增加 `Secure`。
7. session 到期、退出或服务重启后失效。

#### API

| 方法 | 路径 | 是否登录 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/auth/login` | 否 | 登录 |
| `POST` | `/api/auth/logout` | 是 | 删除 session 并清 Cookie |
| `GET` | `/api/auth/session` | 否 | 返回当前是否登录及用户名，不泄露 session |
| `GET` | `/healthz` | 否 | 仅返回服务健康状态，不暴露 workspace 信息 |

#### 防护

- 登录接口按客户端 IP 做滑动窗口或 token bucket 限流，例如每分钟 5 次、短时突发 10 次。
- 不在日志中记录密码、Cookie、session token 或文件正文。
- 修改状态的接口只接受 `application/json`，并校验 `Origin`/`Host`。
- 退出后立即使 session 失效。
- 定时清理过期 session，限制 session 总数，避免内存无限增长。

### 6.3 Workspace 路径安全

所有文件 API 只接受 `/` 分隔的 workspace 相对路径，例如：

```text
book1/chapter1.md
book1/imgs/image1.jpg
```

不接受客户端传入服务器绝对路径。统一路径解析函数执行：

1. 拒绝空字节、绝对路径、盘符路径和不合法编码。
2. URL 解码只由标准库完成一次，不做危险的重复解码。
3. `filepath.FromSlash` 后调用 `filepath.Clean`。
4. 与 workspace root 连接后解析实际符号链接。
5. 使用 `filepath.Rel` 再次检查真实目标仍位于 workspace 内。
6. `rel == ".."` 或以 `../` 开头时返回 `403`。
7. workspace 内部符号链接可访问；指向 workspace 外部的符号链接一律拒绝。
8. 读取前重新检查文件类型，目录接口不读取文件，文件接口不接受目录。

该函数必须被目录、文本、元信息和 raw 文件接口共同使用，避免各接口产生不同的安全边界。

### 6.4 文件 API

采用懒加载目录，而不是启动时递归扫描整个 workspace，以提高大目录和移动网络下的首屏速度。

| 方法 | 路径 | 参数 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/fs/list` | `path`，根目录为空 | 返回指定目录的直接子项 |
| `GET` | `/api/fs/meta` | `path` | 返回文件名、大小、修改时间、MIME、预览类型 |
| `GET` | `/api/fs/text` | `path` | 返回 Markdown、txt、log 等文本内容 |
| `GET` | `/api/fs/raw` | `path` | 流式返回允许内联的图片；其他文件默认下载 |

建议目录项响应：

```json
{
  "path": "book1/chapter1.md",
  "name": "chapter1.md",
  "kind": "file",
  "previewKind": "markdown",
  "size": 12842,
  "modifiedAt": "2026-07-20T10:20:30Z"
}
```

实现规则：

- 目录永远排在文件之前，同类按名称自然排序。
- 隐藏文件也列出，以满足“workspace 下所有文件可访问”；未来可增加排除规则配置。
- 文件类型识别同时参考扩展名和 `http.DetectContentType`，但不信任扩展名决定安全响应头。
- `.md`、`.markdown` 进入 Markdown viewer。
- 常见文本扩展名（`.txt`、`.log`、`.json`、`.yaml`、`.yml`、`.xml`、`.csv`、源码等）进入 Text viewer。
- JPG、PNG、GIF、WebP、BMP、AVIF 等安全图片进入 Image viewer。
- SVG 第一阶段不作为可信 HTML 执行；仅以 `<img>` 沙箱化展示或下载，raw 响应附加严格 CSP。
- 其他文件显示元信息和下载按钮，保证“可访问”但不错误地当作文本载入。
- raw 接口使用 `http.ServeContent`，支持 `Range`、`Last-Modified`、`ETag`/条件请求，不一次性读入内存。
- 文本内容超过默认 10 MiB 时不直接加载，返回明确的 `413` 或结构化错误，并允许下载原文件。
- 初版保证 UTF-8、UTF-8 BOM 和 ASCII；遇到非法 UTF-8 时返回编码警告而不是静默乱码。UTF-16/GB18030 自动识别可列入增强阶段。

### 6.5 HTTP 安全响应头

全局设置：

- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: same-origin`
- `X-Frame-Options: DENY`
- `Permissions-Policy` 禁用不需要的摄像头、麦克风、定位等权限
- CSP 至少限制为同源脚本、样式、图片和字体，禁用 `object-src`、`base-uri`、`frame-ancestors`
- API 响应使用 `Cache-Control: no-store` 或适当的私有缓存策略

raw 文件接口必须防止 workspace 中的 HTML/JS 以同源页面执行：

- 仅已确认的图片 MIME 使用 `Content-Disposition: inline`。
- HTML、JS、SVG 等主动内容默认 `attachment`，或附加 `Content-Security-Policy: sandbox`。
- 文件名使用标准库安全生成 `Content-Disposition`。

### 6.6 稳定性与可观测性

- 使用 `http.Server` 配置 `ReadHeaderTimeout`、`ReadTimeout`、`IdleTimeout` 和合理 header 上限。
- 对 JSON 登录请求设置较小 `MaxBytesReader`。
- 捕获 handler panic，返回统一错误，同时记录 request ID。
- 使用 `slog` 输出结构化日志：状态码、耗时、方法、路由、客户端 IP；不记录查询中的完整敏感路径正文。
- 监听 `SIGINT`/`SIGTERM`，在超时内优雅关闭。
- 错误响应统一为 `{ "code": "...", "message": "..." }`，前端可稳定展示。

## 7. 前端详细设计

### 7.1 登录页

- 简洁居中的登录卡片，移动端宽度使用 `min(100% - 32px, 400px)`。
- 用户名默认填入 `admin`，但允许后端配置其他名称。
- 密码框使用 `autocomplete="current-password"`，支持密码管理器。
- 提交按钮高度至少 44px；键盘弹出时页面仍可滚动和提交。
- 显示加载态和通用错误，不区分“用户名错误”与“密码错误”。
- 应用启动先请求 `/api/auth/session`；未登录显示登录页，已登录直接进入 Reader。
- 任意 API 返回 `401` 时清空前端认证状态并回到登录页。

### 7.2 桌面三栏布局

宽度大于等于约 `1024px` 时：

```text
┌──────────────────────── Toolbar ────────────────────────┐
│ 文件树  主题  当前文件名                      大纲  退出 │
├──────────────┬──────────────────────────┬───────────────┤
│              │                          │               │
│ File Tree    │     Preview Pane         │ Markdown TOC  │
│ 可折叠/隐藏   │     独立滚动              │ 滚动联动       │
│              │                          │               │
└──────────────┴──────────────────────────┴───────────────┘
```

- 左栏默认约 280px，右栏默认约 240px。
- 左栏和右栏均可隐藏；选择持久化到 `localStorage`。
- 桌面端支持拖动分隔条调整栏宽，并设置合理最小/最大宽度。
- 中间栏 `min-width: 0`，避免长代码或 Mermaid 撑破布局。
- 三栏滚动互相独立；页面本身不产生额外滚动条。
- 顶栏只保留阅读必需操作：文件树、当前文件、主题、大纲、刷新、退出。

### 7.3 移动端布局

小于约 `768px` 时不强行并排三栏：

- 主界面只显示中间阅读区。
- 顶部固定/粘性工具栏提供“文件”和“大纲”图标按钮。
- 文件树从左侧滑入，大纲从右侧滑入，宽度约 `min(88vw, 360px)`。
- 两个抽屉互斥，打开一个时关闭另一个。
- 点击遮罩、按 Escape、选择文件或大纲项后关闭抽屉。
- 使用 `100dvh` 并兼容回退；顶栏和抽屉考虑 `env(safe-area-inset-*)`。
- 所有主要触控区域至少 44×44px，树节点行高不小于 40px。
- Markdown 正文字号默认不小于 16px，左右 padding 约 16px。
- 表格、代码块、公式、Mermaid 在自身容器内横向滚动，不导致整个页面横向滚动。
- 图片使用 `max-width: 100%; height: auto`；图片查看器使用 `object-fit: contain`。
- 支持浏览器原生双指缩放，不设置禁止缩放的 viewport。
- 抽屉打开时锁定背景滚动并正确管理焦点，兼顾键盘和屏幕阅读器。

`768px–1023px` 的平板横屏可根据实际空间选择单侧常驻、另一侧抽屉；首版可统一使用抽屉以降低布局复杂度。

### 7.4 文件树

- 根目录首次只请求 `/api/fs/list?path=`。
- 目录第一次展开时才加载直接子项，加载后缓存。
- 提供刷新按钮；刷新当前目录时保留可恢复的展开状态。
- 当前文件高亮，并在重新打开抽屉时尽量滚动到当前项。
- 根据预览类型显示目录、Markdown、文本、图片和通用文件图标。
- 文件名过长时省略，长按/hover 显示相对路径。
- 请求切换时使用 `AbortController` 取消无效请求，避免快速点击造成旧响应覆盖新文件。
- 目录加载失败只影响该节点，可单独重试，不清空整棵树。

### 7.5 预览分发

`PreviewPane` 根据后端的 `previewKind` 分发：

| `previewKind` | 组件 | 行为 |
| --- | --- | --- |
| `markdown` | `MarkdownViewer` | 拉取文本，解析 Markdown，生成大纲并渲染增强内容 |
| `text` | `TextViewer` | `<pre>` 或虚拟化文本视图；移动端默认换行，可切换不换行 |
| `image` | `ImageViewer` | 直接使用受认证的 `/api/fs/raw` URL，居中缩放 |
| `unsupported` | `UnsupportedViewer` | 显示名称、类型、大小、修改时间和下载操作 |

切换文件时显示轻量 loading skeleton；失败时提供错误信息和重试按钮。保留每个已访问文件的滚动位置是增强项，首版至少保证同一文件的大纲跳转准确。

### 7.6 Markdown 渲染链路

渲染顺序：

1. 从 `/api/fs/text` 获取原始 Markdown 和当前文件相对路径。
2. 使用 `markdown-it` 解析，默认关闭原始 HTML（`html: false`）。
3. 自定义 heading rule 生成稳定、唯一的 slug，同时输出大纲数据。
4. fenced code block 中：
   - 语言为 `mermaid` 时保留为待渲染节点。
   - 已注册语言交给 `highlight.js`。
   - 未知语言进行 HTML 转义后按纯文本显示。
5. 自定义 math rule 识别：
   - `$...$`
   - `$$...$$`
   - `\(...\)`
   - `\[...\]`
6. 通过 `DOMPurify` 对最终 HTML 做二次清理，只允许需要的标签、class 和 `data-*` 属性。
7. 插入 DOM 后重写 workspace 相对图片和内部链接。
8. 动态加载 KaTeX 并渲染数学节点。
9. 动态加载 Mermaid，逐块渲染图表。
10. 启动大纲 scroll-spy；图片加载完成后重新计算必要位置。

安全要求：

- Markdown 原始 HTML 默认禁用，避免 workspace 文档注入脚本。
- 外部链接使用 `target="_blank" rel="noopener noreferrer"`。
- 不执行 Markdown 中的 JavaScript URL、事件处理器或 iframe。
- KaTeX 使用 `throwOnError: false`，单个公式错误显示原文/错误标记。
- Mermaid 使用严格安全模式；单图解析错误显示该图错误，不影响其他内容。
- Mermaid 每次渲染绑定 generation ID；用户快速切换文件后，旧异步结果不得覆盖新页面。
- 限制单个 Mermaid block 和单文件 Mermaid 数量，防止超大图表冻结移动浏览器。

### 7.7 相对图片与内部链接

以 `book1/chapter1.md` 为例：

```markdown
![示例](./imgs/image1.jpg)
[下一章](./chapter2.md#第二节)
```

前端重写逻辑：

- 当前文档目录为 `book1/`。
- `./imgs/image1.jpg` 归一化为 `book1/imgs/image1.jpg`。
- 图片地址重写为 `/api/fs/raw?path=<encodeURIComponent(path)>`。
- `./chapter2.md#第二节` 点击后在应用内打开目标文件，并在完成渲染后跳到标题。
- `#本页标题` 只在当前 Markdown 内平滑滚动。
- `http:`、`https:`、`mailto:`、`tel:` 等外部链接不按 workspace 路径处理。
- 前端只做用户体验层归一化，后端仍必须独立执行 workspace 边界检查。
- URL query、fragment 和百分号编码分别处理，避免把 `#` 错当成文件名的一部分。

### 7.8 大纲目录

- 从 Markdown parser 的 heading tokens 生成 H1–H6 列表，避免从未清理的 HTML 字符串猜测。
- 重复标题生成唯一 ID，例如 `intro`、`intro-2`。
- 缩进按文档中出现的最小 heading level 计算。
- 使用 `IntersectionObserver` 或节流后的 scroll-spy 高亮当前章节。
- 点击目录平滑滚动，并在 URL/history 中可选保存 hash。
- 移动端点击目录项后自动关闭大纲抽屉。
- 非 Markdown 文件不显示空大纲栏；桌面端可自动折叠或显示“当前文件无大纲”。

### 7.9 Mermaid 支持范围

以安装并锁定的 Mermaid 稳定版本为准，测试至少覆盖：

- Flowchart
- Sequence diagram
- Class diagram
- State diagram
- Entity relationship diagram
- User journey
- Gantt
- Pie chart
- Git graph
- Mindmap
- Timeline
- Quadrant chart
- XY chart（若所选稳定版本支持）

主题切换后重新渲染 Mermaid，使图中文字、线条和背景与当前主题一致。大图在容器内部缩放/横向滚动，不能撑破阅读区。

### 7.10 日间/夜间主题

- 使用 CSS custom properties 统一控制 shell、正文、边框、代码、表格、引用、滚动条和高亮色。
- 首次访问默认读取 `prefers-color-scheme`。
- 用户手动选择 `light` 或 `dark` 后写入 `localStorage` 并优先于系统设置。
- 在 `<html data-theme="...">` 上切换，避免组件内分散维护主题状态。
- 页面初始化时尽早应用保存主题，减少白屏闪烁。
- 同步选择对应的 highlight.js CSS 和 Mermaid theme。
- 日间和夜间主题都检查 WCAG 可读性，尤其是正文、次要文字、当前目录项和代码注释。

## 8. API 错误约定

示例：

```json
{
  "code": "file_too_large",
  "message": "File exceeds the configured preview limit"
}
```

建议错误码：

- `invalid_request`
- `invalid_credentials`
- `rate_limited`
- `unauthorized`
- `invalid_path`
- `outside_workspace`
- `not_found`
- `not_a_directory`
- `not_a_file`
- `unsupported_preview`
- `file_too_large`
- `invalid_text_encoding`
- `internal_error`

前端对用户显示友好消息；服务端日志保留 request ID 和内部错误上下文，但不把绝对服务器路径、堆栈或配置泄露给浏览器。

## 9. 开发阶段与任务清单

### 阶段 0：项目骨架与工程规范

目标：前后端可独立开发，建立可重复的构建流程。

- [x] 初始化 `go.mod`、Go 目录结构和基础 `main`。
- [x] 初始化 Vue 3 + TypeScript + Vite。
- [x] 配置 Vite 开发代理：`/api` 指向本地 Go 服务。
- [x] 配置生产构建输出到 `internal/webui/dist`。
- [x] 添加 `Makefile`：`dev-backend`、`dev-frontend`、`test`、`build`、`clean`。
- [x] 添加 `.gitignore`，忽略前端依赖和生成产物。
- [x] 添加基础 lint/format：`gofmt`、`go vet`、ESLint、Prettier。
- [x] 建立 `testdata/workspace`，包含需求中的目录和示例文件。

完成条件：前端开发服务器可访问，Go 服务有 `/healthz`，生产命令可以生成单个二进制。

### 阶段 1：配置、认证和服务安全基线

目标：先建立安全边界，再开放文件能力。

- [x] 实现配置读取、默认 `:8848`、workspace 启动校验。
- [x] 实现 `hash-password` 子命令。
- [x] 实现 admin 登录、session 查询、退出。
- [x] 实现 session 过期清理和登录限流。
- [x] 实现认证 middleware，保护全部 `/api/fs/*`。
- [x] 实现安全响应头、请求 ID、结构化日志和 panic recovery。
- [x] 实现 HTTP server timeout 和优雅关闭。
- [x] 完成登录页及全局 `401` 处理。

完成条件：未登录无法获取任何文件信息；登录、刷新 session 状态和退出流程完整。

### 阶段 2：安全文件访问 API

目标：完整支持只读 workspace 浏览。

- [x] 实现唯一的安全路径解析函数。
- [x] 覆盖 `../`、绝对路径、编码路径、符号链接逃逸等测试。
- [x] 实现懒加载目录 API 和自然排序。
- [x] 实现文件元信息及 `previewKind` 判断。
- [x] 实现有限大小文本读取和编码错误处理。
- [x] 实现支持 Range/缓存协商的 raw 文件流。
- [x] 实现主动内容下载策略和 raw CSP。
- [x] 统一 JSON 错误格式。

完成条件：示例 workspace 中所有项可列出，支持类型可预览，其他文件可下载，任何方式都不能读取 workspace 外文件。

### 阶段 3：响应式 Reader Shell 与文件树

目标：先打通移动端优先的浏览和导航。

- [x] 实现桌面三栏 `AppShell`。
- [x] 实现左/右栏显示隐藏及栏宽持久化。
- [x] 实现移动端左右抽屉、遮罩、焦点和背景滚动管理。
- [x] 实现递归文件树和目录懒加载。
- [x] 实现当前项高亮、局部重试和刷新。
- [x] 实现 `PreviewPane` 文件类型分发和 loading/error 状态。
- [x] 实现文本、图片、通用文件查看器。
- [x] 实现主题切换和本地持久化。

完成条件：PC 和手机上均能登录、浏览目录、查看 txt/log/图片并切换主题。

### 阶段 4：Markdown 核心渲染

目标：满足 Markdown reader 的全部必需能力。

- [x] 集成 `markdown-it`，禁用原始 HTML。
- [x] 实现标题 slug、重复标题处理和大纲提取。
- [x] 集成 DOMPurify，并添加 XSS 回归测试。
- [x] 实现相对图片 URL 重写。
- [x] 实现 Markdown 内部文件链接和锚点跳转。
- [x] 集成 highlight.js 和代码块移动端滚动。
- [x] 实现数学分隔符插件并集成 KaTeX。
- [x] 集成 Mermaid、错误隔离、并发 generation 防护和主题重渲染。
- [x] 实现大纲跳转和滚动高亮。
- [x] 完善表格、引用、列表、任务列表、图片、代码、公式的主题样式。

完成条件：统一验收 Markdown 可正确展示相对图片、指定公式语法、代码和目标 Mermaid 图类型。

### 阶段 5：测试、性能和部署

目标：形成可部署、可回归的首个版本。

- [x] 编写 Go handler 集成测试，使用临时 workspace。
- [x] 编写 Vue 组件和 composable 单元测试。
- [x] 编写 Playwright 桌面 Chrome 和移动端 Chrome/WebKit 场景。
- [x] 测试大目录、长文本、大图片、多个 Mermaid 图和快速切换文件。
- [x] 检查键盘导航、焦点、ARIA label、颜色对比度。
- [x] 检查前端 bundle；KaTeX、Mermaid 按需异步加载。
- [x] 编写 README：配置、密码哈希、构建、启动和安全说明。
- [x] 提供 systemd unit、环境文件示例和可选 Dockerfile。
- [ ] 在目标 Linux 服务器进行 8848 端口实机验收。

> 本地验收记录（2026-07-20）：桌面 Chromium 与 390×844 移动 Chromium 场景通过，登录页和 Reader 通过自动化 WCAG A/AA 扫描；1000 项目录、约 1 MiB 长文本、2 MiB 图片 Range、快速文件切换竞态均有自动化测试；12 类 Mermaid 图表画廊全部生成 SVG。移动 WebKit 场景已配置，但当前开发机缺少 GTK/GStreamer 等系统库，需在具备系统依赖的 CI 或目标机复验。

完成条件：自动化测试通过，单二进制能在目标服务器启动，PC 和手机完成端到端验收。

## 10. 测试计划

### 10.1 Go 单元与集成测试

重点测试：

- workspace 根目录和多层目录列表。
- 空 path、`.`、`..`、`../secret`、绝对路径、双重编码、反斜杠路径。
- 指向 workspace 内部和外部的符号链接。
- 文件被并发删除/替换时返回受控错误。
- 登录成功、失败、限流、session 过期、退出。
- 未登录访问 list/text/raw 均返回 `401`。
- txt/md 大小边界和非法 UTF-8。
- raw 的 MIME、`nosniff`、Range、条件请求和下载 header。
- SPA fallback 不吞掉 `/api/*` 的 404。

### 10.2 前端单元测试

重点测试：

- 文件类型到 viewer 的分发。
- 文件树目录首次展开、缓存、刷新和错误重试。
- `./imgs/a.jpg`、`../imgs/a.jpg`、带空格/中文/`#` 的相对路径重写。
- workspace 越界相对路径不生成可用预览 URL。
- `$$...$$` 和 `\(...\)` 渲染。
- fenced code 和未知语言回退。
- Mermaid 成功、单块失败、文件快速切换后的过期渲染丢弃。
- `<script>`、`onerror`、`javascript:` 等恶意 Markdown 被清除。
- 重复标题 slug 和大纲激活状态。
- 主题保存、恢复和 Mermaid 重渲染。

### 10.3 E2E 场景

桌面视口建议 `1440×900`，移动端至少覆盖 `390×844` 和较窄的 `360×800`：

1. 未登录打开首页，进入登录页。
2. 错误密码显示通用错误，正确密码进入 Reader。
3. 展开 `book1`，打开 `chapter1.md`。
4. 验证相对图片、公式、代码和 Mermaid 可见。
5. 点击大纲跳转并高亮当前章节。
6. 打开 `.txt`、`.log` 和图片文件。
7. 切换日间/夜间主题并刷新页面验证持久化。
8. 桌面隐藏/显示左右栏并拖动栏宽。
9. 移动端使用抽屉选择文件和目录，正文无页面级横向滚动。
10. session 失效后下一次 API 请求返回登录页。
11. 退出后浏览器后退也不能重新读取受保护文件。

### 10.4 建议验证命令

```bash
pnpm --dir web install --frozen-lockfile
pnpm --dir web lint
pnpm --dir web test
pnpm --dir web build
go test ./...
go vet ./...
go build ./cmd/web-reader
pnpm exec playwright test
```

最终以项目实际脚本为准，`Makefile` 应提供等价的一键命令。

## 11. 部署计划

### 11.1 单二进制 + systemd（推荐）

- 构建前端并嵌入 Go 二进制。
- 将二进制安装到 `/opt/web-reader/web-reader`。
- 将密码哈希等配置放入权限为 `0600` 的 `/etc/web-reader/web-reader.env`。
- systemd 使用专用低权限用户运行。
- workspace 仅授予该用户读取权限，不授予写权限。
- `ExecStart` 指定 `--addr 0.0.0.0:8848 --workspace /srv/books`。
- 配置 `Restart=on-failure`、文件描述符限制和基础 hardening。
- 防火墙只向需要的来源开放 8848。

### 11.2 Docker（可选）

- 多阶段构建：Node 构建前端，Go 构建静态二进制。
- 运行镜像使用非 root 用户。
- workspace 以只读卷挂载：`-v /data/books:/workspace:ro`。
- 映射端口：`-p 8848:8848`。
- 通过 secret/环境文件注入密码哈希，不写入镜像。

### 11.3 HTTP 安全提醒

需求中的 `http://remote_ip:8848` 可以直接实现，但公网 HTTP 会以明文传输登录凭据和 session Cookie。生产环境强烈建议至少采用以下一种方式：

1. 在 Nginx/Caddy 后配置 HTTPS，并把 `WEB_READER_SECURE_COOKIE=true`。
2. 仅通过 WireGuard/Tailscale/VPN 内网访问。
3. 用防火墙限制 8848 只允许可信 IP。

如果直接暴露 HTTP 到公网，即使应用自身认证正确，也无法抵御链路窃听或 Cookie 劫持。

## 12. 性能策略

- 目录懒加载，避免一次递归返回整个 workspace。
- 文本预览设置大小上限；图片和下载使用流式响应。
- KaTeX 和 Mermaid 动态 import，首屏不加载重型库。
- highlight.js 只注册常用语言，其他语言按纯文本显示。
- 对文件请求使用 `AbortController`，快速切换时取消旧请求。
- Mermaid 逐块渲染并限制输入规模；错误不会阻塞正文。
- 文件列表和已读取的小文本可做有限的内存/浏览器缓存，以 `mtime` 或 ETag 失效。
- scroll-spy 使用 `IntersectionObserver` 或 `requestAnimationFrame` 节流。
- 图片设置尺寸约束，避免解码后撑破布局；超大图片依赖浏览器解码但不复制到 JS 内存。

## 13. 首版明确不做的功能

以下内容不属于当前需求，首版不实现：

- 多用户、角色和权限管理。
- 文件编辑、保存、上传、删除、移动和重命名。
- 全文搜索。
- 多标签页、阅读历史跨设备同步。
- PDF/DOCX 导出和打印排版优化。
- workspace 在线切换。
- 文件系统实时 watcher 和服务端推送。
- 数据库持久化 session。
- Markdown 中任意原始 HTML 或自定义脚本执行。

这些功能应在安全的只读 MVP 稳定后再评估。

## 14. 主要风险与应对

| 风险 | 影响 | 应对 |
| --- | --- | --- |
| 路径穿越/符号链接逃逸 | 读取服务器敏感文件 | 所有接口复用真实路径校验；专项单元测试 |
| HTTP 明文传输 | 密码和 Cookie 被窃听 | 推荐 HTTPS/VPN/防火墙；HTTPS 时 Secure Cookie |
| 恶意 Markdown XSS | session 或同源数据泄露 | 禁用 raw HTML、DOMPurify、CSP、严格 URL scheme |
| workspace 中 HTML/SVG 主动内容 | 同源脚本执行 | raw 类型白名单、attachment、sandbox CSP、nosniff |
| 超大文本/目录/Mermaid | 内存占用或手机页面卡死 | 懒加载、大小上限、流式响应、渲染数量限制 |
| 快速切换文件产生异步竞态 | 旧 Mermaid 覆盖新内容 | AbortController + render generation ID |
| 主题切换后 Mermaid 颜色错误 | 夜间阅读体验差 | 保存源码，主题变化后重渲染图表 |
| 文本编码不一致 | txt/log 乱码 | 首版明确 UTF-8；非法编码提示；后续增加编码检测 |
| session 仅在内存 | 服务重启需重新登录 | 单用户场景可接受；在 README 明确说明 |

## 15. 首版完成定义（Definition of Done）

只有同时满足以下条件才视为首版完成：

- [x] 可配置 workspace、admin 密码哈希，默认监听 `0.0.0.0:8848`。
- [x] 所有文件 API 均受登录保护且无法逃逸 workspace。
- [x] 桌面端三栏可用，左右栏可隐藏。
- [x] 移动端使用左右抽屉，正文阅读和触控操作顺畅。
- [x] 文件树可浏览 workspace 下所有目录和文件。
- [x] `.md`、`.txt`、`.log`、常见图片可直接查看。
- [x] Markdown 相对图片正确显示。
- [x] `$$...$$`、`\(...\)` 公式通过验收。
- [x] 代码块和目标 Mermaid 类型通过验收。
- [x] 大纲可跳转并随滚动高亮。
- [x] 日间/夜间主题完整覆盖 shell 和正文并可持久化。
- [x] Go、前端单元测试和关键 Playwright 场景通过。
- [x] 构建得到可在目标 Linux 服务器运行的单二进制。
- [x] README 和 systemd 部署示例完整，明确 HTTP 安全风险。

## 16. 推荐实施顺序总结

严格按以下顺序推进，避免先完成漂亮 UI 后再返工安全边界：

1. 工程骨架和单二进制构建。
2. 配置、登录、session 和安全 middleware。
3. workspace 路径校验及文件 API。
4. 移动优先 Reader shell、文件树和基础预览。
5. Markdown、相对资源、公式、代码、Mermaid 和大纲。
6. 主题细化、自动化测试、性能检查和远程部署验收。

预计工作量可拆为 5 个主要开发里程碑；具体工期取决于目标浏览器范围、workspace 规模以及是否要求首版支持非 UTF-8 编码。优先级上，路径安全、认证、移动端导航和 Markdown 核心渲染均为 P0，编码自动检测、滚动位置恢复和 Docker 为 P1。

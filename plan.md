# Web Reader 代码审查修复计划

基于 2026-10-13 代码审查报告，按优先级执行以下修复。

## P0：确认的功能缺陷

### 1. 修复 `terminalEnabled` 数据竞争 [REPRO]
- 文件：`internal/server/server.go`
- 现状：`*bool` 裸指针分发给 3 个 handler，写（`setSettingsHandler`）与读（`terminalHandler`/`getSettingsHandler`）无同步，`-race` 已复现。
- 方案：改用 `sync/atomic.Bool`；同时删除从未读取的 `Server.config`、`Server.terminalEnabled` 死字段。
- 验证：`go test -race ./internal/server/` 通过。

### 2. 修复 `Apply` 不检查 `HasUpdate` [CODE]
- 文件：`internal/updater/service.go`、`internal/server/server.go`
- 现状：已是最新版仍会下载、替换二进制并重启。
- 方案：`Apply` 中 `Check` 后若 `!HasUpdate` 返回 `ErrNoUpdateFound`（顺带启用该死错误变量）；`applyUpdateHandler` 映射为 409。
- 验证：`go test ./internal/updater/ ./internal/server/` 通过。

## P1：死代码清理

### 3. 删除 Go 死代码
- `internal/filesystem/browse.go`：删除无调用方的 `SplitPathSegments`（deadcode 确认 unreachable）。
- `internal/version/version.go`：删除从未读取的 `GitCommit`、`BuildDate`；默认 `Version` 改为 `"dev"`（防止未注入 ldflags 的本地构建误报 v1.8.0 并触发自更新，`Get()` 简化为直接返回）。
- `cmd/web-reader/main.go`：去掉 `app.HTTPServer()` 重复调用（`srvRef`/`httpServer` 合一）。

### 4. 删除前端未使用图标
- 文件：`web/src/utils/icons.ts`
- 删除 10 个未引用图标：`home/plus/hash/quote/heading/list-ordered/function/network/alert-triangle/chevrons-down-up`（后者与 `collapse-all` 内容重复）。
- 验证：grep 确认无引用；`pnpm typecheck && pnpm test`。

## P2：重复代码合并

### 5. 合并 same-origin 校验双份实现
- 现状：`internal/auth/handler.go:83-117` 与 `internal/server/middleware.go:142-176` 逐字重复 `matchHost`/`requireSameOrigin`。
- 方案：新建 `internal/httputil` 包导出 `RequireSameOrigin`，auth 与 server 共用，删除两份私有副本（保留 handler 层 defense-in-depth 调用）。

### 6. 合并工作区路径解析重复
- `internal/filesystem/service.go` `SetRoot`：改为调用 `config.ResolveWorkspaceDir`（逻辑等价：tilde 展开→Abs→创建→EvalSymlinks→敏感路径→目录校验），删除内联副本。
- 提取 `createEntry`/`SaveUpload` 共用的"父目录解析+包含校验"为私有辅助函数。

### 7. 前端重复逻辑收敛
- 新建 `web/src/utils/clipboard.ts`（`copyToClipboard`）；`FileTree.vue` 提取 `copyAbsolutePath` 辅助函数替换 5 处复制体，`DocumentTabs.vue` 同步替换。
- `DocumentTabs.vue` 删除 `tabIcon`，改用 `fileIconName`（签名放宽为 `Pick<FsItem,'kind'|'previewKind'>`）。
- `SettingsModal.vue` 删除本地 `formatBytes`，改用 `utils/format.ts`。
- 新建 `web/src/utils/timers.ts`（`createTimerRegistry`），替换 `MarkdownViewer.vue` 与 `CodeViewer.vue` 中重复的 `scheduleTimeout/pendingTimers`。
- `FileTree.vue:74` 恒真条件简化为直接赋值。
- `web/src/utils/prism.ts`：底部 import 移到顶部，删除 `escapeHtml` 再导出与 default 导出；`CodeViewer.vue` 改从 `utils/escape` 导入。

## P3：设计与一致性

### 8. 版本号单一真源
- `web/vite.config.ts`：从 `package.json` 读取版本注入 `__APP_VERSION__`；`env.d.ts` 声明；`SettingsModal.vue` 删除硬编码 `'v1.8.0'`。
- （`version.go` 默认值在 P1-3 中一并处理；Makefile ldflags 注入逻辑不变。）

### 9. `filesystem.Service.Events` 封装
- 字段改为私有 `events`，新增 `SubscribeEvents`/`UnsubscribeEvents` 方法；`server.go` eventsHandler 改用方法。

## 明确不处理（本轮范围外）

- updater 的 4 个 `With*` functional options 与 `SetupDetachedProcess` 平台抽象：判定为可接受的惯例写法。
- 错误信息中英文混用：涉及面大、收益低，留待后续统一 i18n 时再处理。
- `ReaderView.vue` 的 localStorage JSON 助手与 `utils/storage.ts` 的合并：两者语义不同（JSON vs 标量），合并收益小。
- `POST /api/workspace` 自动建目录的副作用：属产品行为变更，需另行确认。

## 验证步骤

1. `gofmt -l cmd internal`、`go vet ./...`
2. `go test ./...` 全绿；`go test -race ./internal/server/` 无竞争告警
3. `pnpm --dir web lint`、`typecheck`、`test` 全绿
4. 回归确认 `deadcode` 不再报告 `SplitPathSegments`

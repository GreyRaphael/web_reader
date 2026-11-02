# Web Reader v1.8.2

Web Reader 是一个轻量、现代化、全功能包含的 Web 文件树阅读与 Markdown 编辑器。基于 Go 1.24 和 Vue 3 构建，前端生产资源全量内嵌在单个 Go 二进制文件中。

### 🛠️ 依赖安全与版本升级
- **ESLint 10 核心工具链升级**：将 `eslint` 升级至最新稳定版 `10.14.0`，`@eslint/js` 升级至 `10.0.1`，同步升级 `typescript-eslint`（8.72.1）与 `eslint-plugin-vue`（10.12.0），消除 ESLint 9 EOL 废弃警告。
- **废弃子依赖治理**：
  - 升级 `jsdom` 至 `30.1.3`（底层迁移至 `@exodus/bytes`），彻底移除了已废弃的 `whatwg-encoding`。
  - 升级 `@vue/test-utils` 至 `2.5.1`，并通过工作区 override 将 `test-exclude` 统一至 `8.0.0`，彻底移除存在安全风险的旧版本 `glob@10.5.0`，平滑统一至 `glob@13.0.6`。
- **DOMPurify 升级**：将 `dompurify` 升级至 `3.4.17`，强化富文本渲染的 XSS 防御能力。
- **测试框架升级**：升级 `@playwright/test` 至 `1.64.1`。

### 🧹 代码质量与检查优化
- **WikiLinks 插件清理**：根据 ESLint 10 新增的 `no-useless-assignment` 规则，优化 `wikilinks.ts` 中 `targetPath` 的初始化逻辑，消除冗余赋值。
- **Release 工作流优化**：重构 GitHub Actions 发布工作流，支持自动展示中文 Release 说明，修复矩阵多构建任务导致日志重复追加的问题。

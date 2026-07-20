# Web Reader

Web Reader 是一个部署在 Linux 服务器上的只读 workspace 阅读器。它使用 Go 提供认证与安全文件 API，并把 Vue 3 前端嵌入单个可执行文件中。

## 功能

- 单管理员登录、bcrypt 密码哈希和服务端内存 session。
- 安全限制在固定 workspace 内，拒绝路径穿越和越界符号链接。
- 懒加载文件树，支持 Markdown、文本、日志、常见图片和其他文件下载。
- 桌面三栏布局，移动端文件树与大纲抽屉。
- Markdown 相对图片、内部链接、KaTeX、代码高亮、Mermaid 和文章大纲。
- 日间、夜间和跟随系统主题。
- 前端生产资源嵌入 Go 二进制，无需服务器安装 Node.js。

首版严格只读，不提供上传、编辑、删除、移动或重命名能力。

## 环境要求

- Go 1.24（`go.mod` 中的 toolchain 为 `go1.24.4`）。
- Node.js 20+。
- pnpm 11。项目默认使用 `/home/gewei/.local/share/pnpm/bin/pnpm`，可通过 `make PNPM=/path/to/pnpm ...` 覆盖。

## 安装与验证

```bash
make install
make lint
make test
make build
```

`make build` 会先构建前端，再生成静态 Go 二进制：

```text
build/web-reader
```

构建过程中 `internal/webui/dist` 会临时写入生产资源；Go 编译完成后会自动恢复可提交的占位文件。不要直接发布只执行 `go build` 得到的二进制，因为它可能只包含占位页面。

### E2E

先安装 Playwright 浏览器：

```bash
/home/gewei/.local/share/pnpm/bin/pnpm --dir web exec playwright install chromium firefox webkit
make test-e2e

# 安装完整系统依赖后执行包含桌面 Firefox 和移动 WebKit 的矩阵
/home/gewei/.local/share/pnpm/bin/pnpm --dir web test:e2e:all
```

WebKit 还需要 Playwright 列出的 GTK、GStreamer 等系统库。在 Debian/Ubuntu 上可由具备 sudo 权限的管理员执行：

```bash
/home/gewei/.local/share/pnpm/bin/pnpm --dir web exec playwright install-deps chromium firefox webkit
```

## 创建管理员密码哈希

使用交互输入，避免明文密码进入 shell history：

```bash
./build/web-reader hash-password
```

命令会把 bcrypt 哈希输出到标准输出。将哈希写入受限环境文件，不要提交到仓库。

## 启动

```bash
export WEB_READER_WORKSPACE=/srv/books
export WEB_READER_ADMIN_PASSWORD_HASH='$2a$...'
./build/web-reader
```

默认访问地址为 `http://server-ip:8848`，健康检查为：

```bash
curl http://127.0.0.1:8848/healthz
```

也可使用命令行参数：

```bash
./build/web-reader \
  --addr 0.0.0.0:8848 \
  --workspace /srv/books \
  --admin-user admin \
  --password-hash '$2a$...'
```

命令行参数优先于环境变量。

## 配置

| 参数 | 环境变量 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--addr` | `WEB_READER_ADDR` | `0.0.0.0:8848` | HTTP 监听地址 |
| `--workspace` | `WEB_READER_WORKSPACE` | 无 | 必填，只读 workspace |
| `--admin-user` | `WEB_READER_ADMIN_USERNAME` | `admin` | 管理员用户名 |
| `--password-hash` | `WEB_READER_ADMIN_PASSWORD_HASH` | 无 | 必填，bcrypt 哈希 |
| `--session-ttl` | `WEB_READER_SESSION_TTL` | `24h` | 内存 session 有效期 |
| `--max-text-size` | `WEB_READER_MAX_TEXT_SIZE` | `10MiB` | 文本在线预览上限 |
| `--secure-cookie` | `WEB_READER_SECURE_COOKIE` | `false` | HTTPS 部署时设为 `true` |

Session 保存在进程内存中，服务重启后用户需要重新登录。

## 本地开发

先准备 workspace 和密码哈希环境变量，然后分别启动后端与前端：

```bash
make dev-backend
make dev-frontend
```

Vite 会把 `/api` 代理到 `http://localhost:8848`。

## systemd 部署

示例文件位于：

- `deploy/web-reader.service`
- `deploy/web-reader.env.example`

推荐步骤：

```bash
sudo useradd --system --home /nonexistent --shell /usr/sbin/nologin web-reader
sudo install -d -o root -g root /opt/web-reader
sudo install -m 0755 build/web-reader /opt/web-reader/web-reader
sudo install -d -o root -g web-reader -m 0750 /etc/web-reader
sudo install -o root -g web-reader -m 0640 deploy/web-reader.env.example /etc/web-reader/web-reader.env
sudo install -m 0644 deploy/web-reader.service /etc/systemd/system/web-reader.service
sudo systemctl daemon-reload
sudo systemctl enable --now web-reader
```

编辑 `/etc/web-reader/web-reader.env`，设置真实 workspace 和密码哈希。运行用户只需要 workspace 的读取与目录遍历权限。

## 安全说明

- 直接通过公网 HTTP 访问会明文传输密码和 session Cookie。生产环境应使用 Caddy/Nginx HTTPS、VPN（如 WireGuard/Tailscale）或严格的防火墙来源限制。
- HTTPS 终止后应设置 `WEB_READER_SECURE_COOKIE=true`。
- 环境文件包含密码哈希，建议权限不高于 `0640`，并限制为 root 与服务组可读。
- workspace 中的 HTML、JavaScript、SVG 等主动内容默认下载；raw 响应使用 sandbox CSP 和 `nosniff`。
- 服务不会跟随指向 workspace 外部的符号链接。
- 不要赋予 `web-reader` 运行用户对 workspace 的写权限。

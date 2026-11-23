# depman

CICD 自动化发布与运维台：按项目管理发布、按环境管理凭据，支持 SSH 执行脚本发布、交互终端与文件/镜像传输（传输与 Git 流水线后续扩展）。

风格对齐 k8man：**默认只读** / **可操作**；8 位字母 key + session token（`X-Igo-Token`）。

## 特性

- **depman**：项目胶囊首页；主题页按环境 SSH 跑脚本发布
- **环境**：按环境管理 Git / Harbor / SSH / API 凭据；密钥 AES 落库，默认不明文
- **交互**：SSH 连通测试（Web 终端后续）
- **传输**：Harbor 镜像互传、SSH 上传下载（后续）
- **登录**：居中输入 key；超管 key 或超管签发的**项目管理 key**（绑定项目/环境）
- 单二进制 + SQLite；可选嵌入前端

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go · Gin · SQLite（modernc） |
| 前端 | Vue3 · Vite · Vue Router |
| SSH | `golang.org/x/crypto/ssh` |

Go module：`depman`（不绑定 gitee.com）。

## 快速开始

### 后端

```bash
go mod tidy
# 可选：先构建前端再嵌入
# cd web && npm install && npm run build && cd .. && mkdir -p cmd/server/webdist && cp -r web/dist/* cmd/server/webdist/
go run ./cmd/server
```

默认监听 `:8080`，数据库 `./data/depman.db`，配置 `./etc/depman.conf`（缺失时自动生成）。

| 变量 | 默认 | 说明 |
|------|------|------|
| `DEPMAN_ADDR` | `:8080` | 监听地址 |
| `DEPMAN_DATA_DIR` | `./data` | 数据目录 |
| `DEPMAN_DB_PATH` | `$DATA_DIR/depman.db` | SQLite |
| `DEPMAN_ETC_DIR` | `./etc` | 配置与可选 icon/banner |

### 前端（开发）

```bash
cd web
npm install
npm run dev   # Vite 代理 /api -> :8080
```

## 鉴权

1. 默认只读：可看项目胶囊与环境摘要
2. `GET /api/v1/auth/igokey` 取得 **项目超管** 8 位字母密钥（约 1 小时；剩余 >15 分钟刷新复用）
3. 「登录」页输入 key → 获得 token；写接口带 `X-Igo-Token`
4. 超管可在登录页签发 **项目管理 key**（绑定 `projectIds` / `envIds`，默认 24h，最长 7 天）
5. 页眉 slogan「practice makes perfect」双击可复制超管 key；双击「只读/可操作」可快速登录

## 发布语义（当前）

对「项目 × 环境」配置 SSH 凭据 + 脚本 + 工作目录，点发布后服务端 SSH 到目标机执行 `bash -lc`。  
Git 构建流水线后续再接。

## 目录

```
cmd/server/           入口（可嵌入 webdist）
internal/auth/        igokey + 会话 + 作用域校验
internal/api/read|write
internal/store/       SQLite
internal/cred/        密钥加密
internal/sshx/        SSH 执行
internal/deploy/      发布任务
web/                  Vue3
etc/                  depman.conf、可选 icon
deploy/               Dockerfile 与 K8s 清单
docs/deploy.md        部署说明
```

## 容器 / 生产部署

- 镜像仓库：`krccr.ccs.tencentyun.com/igokorea/depman`
- 构建机：`VM-4-11-ubuntu`（nerdctl，2C2G 限并行 + cache）
- 运行：`zabbix68`，Namespace / Deployment 均为 `depman`，域名 `depman.igozhang.cn`

步骤见 [deploy/deploy.md](deploy/deploy.md)。

## Logo

内置「CICD」SVG（&lt;2K）作 favicon / 页眉图标；可将自有 `icon.svg` / `icon.png` 放入 `etc` 覆盖。

## 文档

- [部署](deploy/deploy.md)

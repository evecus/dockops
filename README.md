# DockOps

> 现代化 Docker 管理面板，以 Compose 为核心，单二进制部署。

![DockOps](https://img.shields.io/badge/DockOps-v1.0.0-06b6d4?style=for-the-badge)
![Go](https://img.shields.io/badge/Go-1.22-00ADD8?style=for-the-badge&logo=go)
![Vue](https://img.shields.io/badge/Vue-3.4-4FC08D?style=for-the-badge&logo=vue.js)

## 功能特性

- 🐳 **Compose 管理** — 以 docker-compose 为核心管理所有容器
- 📦 **4 种创建方式** — 上传文件 / 粘贴内容 / 解析 docker run / 表单填写
- 🖥️ **Web 终端** — 浏览器直接进入容器终端（xterm.js）
- 📋 **实时日志** — WebSocket 实时日志流，支持搜索、高亮、下载
- 📁 **文件管理** — 容器内文件浏览、上传、下载、删除、预览
- 🔄 **更新检测** — 定时检测镜像更新，一键升级
- 🌐 **网络 & 存储** — 完整的 Docker 网络和 Volume 管理
- 🔒 **单二进制** — 前端静态资源内嵌，无需 Nginx
- 🔐 **HTTPS 支持** — 配置证书即可启用 TLS

## 快速开始

### 下载

```bash
# Linux amd64
curl -L https://github.com/dockops/dockops/releases/latest/download/dockops-linux-amd64 -o dockops
chmod +x dockops

# Linux arm64
curl -L https://github.com/dockops/dockops/releases/latest/download/dockops-linux-arm64 -o dockops
chmod +x dockops
```

### 运行

```bash
# 默认配置（HTTP :8080）
./dockops

# 指定配置文件
./dockops -c config.yaml
```

首次访问 `http://your-server:8080` 将引导创建管理员账号。

### 配置文件

```yaml
# config.yaml
http_port: 8080
https_port: 8443
cert_path: /path/to/cert.pem   # 可选
key_path:  /path/to/key.pem    # 可选
data_path: ./data
```

### 以系统服务运行

```ini
# /etc/systemd/system/dockops.service
[Unit]
Description=DockOps Docker Manager
After=docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/dockops -c /etc/dockops/config.yaml
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
```

```bash
systemctl enable --now dockops
```

## 从源码构建

```bash
git clone https://github.com/dockops/dockops
cd dockops

# 安装依赖并构建
make build

# 运行
./dockops -c config.yaml
```

**依赖：**
- Go 1.22+
- Node.js 20+
- CGO（SQLite）：`gcc` 或 `musl-gcc`

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go + Gin + Docker SDK |
| 前端 | Vue 3 + Vite（无 UI 框架） |
| 数据库 | SQLite（嵌入式） |
| 终端 | xterm.js + WebSocket |
| 日志 | WebSocket 实时流 |
| 认证 | JWT |
| 构建 | GitHub Actions |

## 项目结构

```
dockops/
├── main.go                    # 入口
├── config.yaml                # 配置示例
├── internal/
│   ├── config/                # 配置加载
│   ├── auth/                  # JWT 认证
│   ├── db/                    # SQLite 操作
│   ├── docker/                # Docker SDK 封装
│   ├── compose/               # Compose 文件管理
│   ├── parser/                # docker run 命令解析
│   ├── handler/               # HTTP 路由 & 处理器
│   ├── ws/                    # WebSocket（终端 + 日志）
│   ├── scheduler/             # 定时任务（更新检测）
│   └── middleware/            # JWT 中间件
├── web/                       # Vue 3 前端
│   ├── src/
│   │   ├── views/             # 页面组件
│   │   ├── components/        # 公共组件
│   │   ├── api/               # API 客户端
│   │   ├── stores/            # Pinia 状态
│   │   └── styles/            # 全局样式
│   └── dist/                  # 构建产物（内嵌到二进制）
└── .github/workflows/         # CI/CD
    └── build.yml
```

## License

MIT © DockOps Contributors

# DockPs

> 现代化 Docker 管理面板，以 Compose 为核心，单二进制部署，无需配置文件。

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?style=for-the-badge&logo=go)
![Vue](https://img.shields.io/badge/Vue-3.4-4FC08D?style=for-the-badge&logo=vue.js)

## 功能特性

- 🐳 **Compose 管理** — 以 docker-compose 为核心管理所有容器
- 📦 **4 种创建方式** — 上传文件 / 粘贴内容 / 解析 docker run / 表单填写
- 🖥️ **Web 终端** — 浏览器直接进入容器终端
- 📋 **实时日志** — WebSocket 实时日志流，支持搜索、高亮、下载
- 📁 **文件管理** — 容器内文件浏览、上传、下载、删除
- 🔄 **镜像更新** — 检测镜像是否为最新版本，有更新才拉取
- 🌐 **网络 & 存储** — 完整的 Docker 网络和 Volume 管理
- 🔒 **单二进制** — 前端静态资源内嵌，无需 Nginx、无需配置文件
- 🔐 **HTTPS 自动启用** — 在 cert 目录放置证书文件即可自动开启 TLS

## 快速开始

### 下载

```bash
# Linux amd64
curl -L https://github.com/evecus/Dockps/releases/latest/download/dockps-linux-amd64 -o dockps
chmod +x dockps

# Linux arm64
curl -L https://github.com/evecus/Dockps/releases/latest/download/dockps-linux-arm64 -o dockps
chmod +x dockps
```

### 运行

```bash
# 直接运行，默认 HTTP 9080 端口
./dockps

# 指定端口和数据目录（所有参数可选）
./dockps --http 8080 --https 8443 --dir /opt/dockps/data
```

首次访问 `http://your-server:9080` 将引导创建管理员账号。

### 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--http` | `9080` | HTTP 监听端口 |
| `--https` | `9443` | HTTPS 监听端口（需有证书才生效） |
| `--dir` | 二进制同级 `data/` 目录 | 数据存储目录 |

### 启用 HTTPS

将证书文件放入数据目录下的 `cert/` 文件夹，程序启动时自动检测并开启 HTTPS，无需任何配置：

```
data/
└── cert/
    ├── cert.pem      # 或 fullchain.pem / server.crt
    └── key.pem       # 或 privkey.pem / server.key
```

### 以系统服务运行

```ini
# /etc/systemd/system/dockps.service
[Unit]
Description=DockPs Docker Manager
After=docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/dockps --http 9080 --dir /opt/dockps/data
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
```

```bash
systemctl enable --now dockps
```

## 从源码构建

```bash
git clone https://github.com/evecus/Dockps
cd Dockps

# 构建前端
cd web && npm install && npm run build && cd ..

# 构建二进制
go build -o dockps .

# 运行
./dockps
```

**依赖：**
- Go 1.22+
- Node.js 20+

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go + Gin + Docker SDK |
| 前端 | Vue 3 + Vite（无 UI 框架） |
| 数据库 | SQLite（仅存账号和设置） |
| 终端 | xterm.js + WebSocket |
| 认证 | JWT |

## 项目结构

```
Dockps/
├── main.go
├── internal/
│   ├── config/       # 启动参数与初始化
│   ├── auth/         # JWT 认证
│   ├── db/           # SQLite（账号 & 设置）
│   ├── docker/       # Docker SDK 封装
│   ├── compose/      # Compose 文件管理
│   ├── parser/       # docker run 命令解析
│   ├── handler/      # HTTP 路由 & 处理器
│   ├── ws/           # WebSocket（终端 + 日志）
│   ├── scheduler/    # 定时任务（仪表盘数据采集）
│   └── middleware/   # JWT 中间件
└── web/              # Vue 3 前端
    └── src/
        ├── views/
        ├── components/
        ├── api/
        └── stores/
```

## License

MIT

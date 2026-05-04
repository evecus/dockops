# DockOps Patch 2 — Changed Files

## Files (覆盖到项目对应路径)

| 文件 | 变更 |
|------|------|
| `main.go` | 声明 `Version` 变量，通过 ldflags 注入，传入 Server |
| `internal/handler/server.go` | Server 结构体增加 `version` 字段；`/api/system/status` 返回 `version` |
| `internal/scheduler/scheduler.go` | 删除 "Dashboard data collected and cached" 日志输出 |
| `web/src/styles/global.css` | 侧边栏 logo 字号 17→19px，导航项字号 14→15px，图标放大 |
| `web/src/views/Layout.vue` | 删除顶部栏副标题（breadcrumb） |
| `web/src/views/Settings.vue` | About 卡片加标题"关于"、图标移入卡片内、版本号动态读取；复制命令增加 execCommand fallback |

## 版本号说明
workflow 运行时 `-ldflags="-X main.Version=$TAG"` 已在原 main.yml 中配置，前端通过 `/api/system/status` 接口的 `version` 字段读取并显示。

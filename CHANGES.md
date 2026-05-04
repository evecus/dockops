# DockOps Changes

## Modified Files (place into corresponding project directories)

```
web/src/styles/global.css       — 全局样式重写：更大字体、专业配色、去除炫酷效果
web/src/stores/i18n.js          — 新增：国际化 store，持久化中英切换
web/src/views/Login.vue         — 登录页：增加右上角中英切换按钮，字体变大
web/src/views/Layout.vue        — 主布局：顶部栏增加中英切换按钮，侧边栏文字国际化
web/src/views/Dashboard.vue     — 仪表盘：所有文字国际化，字体变大
```

## Changes Summary

1. **字体变大** — base font-size: 14px → 15px，所有组件字体同步放大
2. **专业现代设计** — 蓝色系（#2563eb）取代青色，去掉发光/渐变等装饰效果
3. **中英切换** — 登录页右上角 + 主界面顶部栏均有切换按钮，选择持久化到 localStorage
4. **i18n Store** — `web/src/stores/i18n.js`，通过 Pinia 管理语言状态

## How to Add to Main.js (if Pinia not already set up)
Already set up in the original project. No changes needed to main.js.

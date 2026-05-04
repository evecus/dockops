<template>
  <div class="app-layout">
    <!-- 移动端遮罩层 -->
    <Transition name="overlay">
      <div v-if="drawerOpen" class="mobile-overlay" @click="drawerOpen = false"></div>
    </Transition>

    <!-- Sidebar -->
    <aside class="sidebar" :class="{ 'drawer-open': drawerOpen }">
      <div class="sidebar-logo">
        <div class="logo-icon">
          <img src="/apple-touch-icon.png" width="20" height="20" style="border-radius:4px;display:block;" alt="DockOps" />
        </div>
        <span class="logo-text">Dock<span>Ops</span></span>
        <!-- 移动端关闭按钮 -->
        <button class="drawer-close" @click="drawerOpen = false">
          <X :size="18" />
        </button>
      </div>

      <nav class="sidebar-nav">
        <RouterLink v-for="item in navItems" :key="item.path"
          :to="item.path" custom v-slot="{ isActive, navigate }">
          <div class="nav-item" :class="{ active: isActive }" @click="() => { navigate(); drawerOpen = false }">
            <component :is="item.icon" :size="17" />
            <span>{{ item.label }}</span>
            <span v-if="item.badge" class="nav-badge">{{ item.badge }}</span>
          </div>
        </RouterLink>
      </nav>

      <div class="sidebar-footer">
        <div class="nav-item" @click="handleLogout">
          <LogOut :size="17" />
          <span>退出登录</span>
        </div>
      </div>
    </aside>

    <!-- Main -->
    <div class="main-content">
      <!-- Topbar -->
      <header class="topbar">
        <div class="topbar-left">
          <!-- 移动端汉堡按钮 -->
          <button class="hamburger-btn" @click="drawerOpen = true">
            <Menu :size="20" />
          </button>
          <div>
            <div class="topbar-title">{{ currentTitle }}</div>
            <div class="topbar-breadcrumb" v-if="currentDesc">{{ currentDesc }}</div>
          </div>
        </div>
        <div class="topbar-actions">
          <!-- 刷新按钮：只在仪表盘显示 -->
          <button v-if="isDashboard"
            class="btn btn-ghost btn-sm topbar-refresh"
            @click="globalRefresh" :disabled="refreshing" title="刷新仪表盘数据">
            <div v-if="refreshing" class="spinner" style="width:12px;height:12px;border-width:2px"></div>
            <RefreshCw v-else :size="13" />
          </button>
          <div class="docker-status" :class="dockerOk ? 'ok' : 'err'">
            <span class="status-dot"></span>
            <span class="docker-status-text">{{ dockerOk ? 'Docker 已连接' : 'Docker 未连接' }}</span>
          </div>
          <div class="topbar-user">
            <div class="user-avatar">
              <User :size="14" />
            </div>
            <span class="user-name">Admin</span>
          </div>
        </div>
      </header>

      <!-- Page content -->
      <div class="page-content">
        <RouterView v-slot="{ Component }">
          <Transition name="page" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { RouterLink, RouterView, useRouter, useRoute } from 'vue-router'
import { LayoutDashboard, Box, Image, Network, Settings, LogOut, User, RefreshCw, Menu, X } from 'lucide-vue-next'
import api from '@/api'

const router = useRouter()
const route = useRoute()
const dockerOk = ref(true)
const refreshing = ref(false)
const drawerOpen = ref(false)

const isDashboard = computed(() => route.path === '/dashboard')

// Shared dashboard data — provided to Dashboard.vue via inject
const dashboardRefreshData = ref(null)
provide('dashboardRefreshData', dashboardRefreshData)

async function globalRefresh() {
  if (route.path !== '/dashboard') return
  refreshing.value = true
  try {
    const res = await api.dashboardRefresh()
    // Push fresh data down to Dashboard via provided ref
    dashboardRefreshData.value = res.data
  } catch {}
  finally { refreshing.value = false }
}

const navItems = [
  { path: '/dashboard', label: '仪表盘', icon: LayoutDashboard },
  { path: '/containers', label: '容器管理', icon: Box },
  { path: '/images', label: '镜像管理', icon: Image },
  { path: '/network-storage', label: '网络 & 存储', icon: Network },
  { path: '/settings', label: '设置', icon: Settings },
]

const pageMeta = {
  '/dashboard': { title: '仪表盘', desc: '主机状态与资源概览' },
  '/containers': { title: '容器管理', desc: '管理 Docker Compose 容器' },
  '/images': { title: '镜像管理', desc: '查看与管理本地镜像' },
  '/network-storage': { title: '网络 & 存储', desc: '网络与卷管理' },
  '/settings': { title: '系统设置', desc: '配置系统参数' },
}

const currentTitle = computed(() => pageMeta[route.path]?.title || 'DockOps')
const currentDesc = computed(() => pageMeta[route.path]?.desc || '')

function handleLogout() {
  localStorage.removeItem('token')
  router.push('/login')
}

onMounted(async () => {
  try {
    await api.dashboardInfo()
    dockerOk.value = true
  } catch { dockerOk.value = false }
})
</script>

<style scoped>
/* ===== 移动端遮罩 ===== */
.mobile-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 199;
  backdrop-filter: blur(2px);
}
.overlay-enter-active, .overlay-leave-active { transition: opacity 0.25s ease; }
.overlay-enter-from, .overlay-leave-to { opacity: 0; }

/* ===== 汉堡按钮（桌面隐藏） ===== */
.hamburger-btn {
  display: none;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--border);
  flex-shrink: 0;
  transition: all var(--transition);
}
.hamburger-btn:hover { color: var(--accent); border-color: var(--accent); }

/* ===== 抽屉关闭按钮（桌面隐藏） ===== */
.drawer-close {
  display: none;
  margin-left: auto;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  border: none;
  flex-shrink: 0;
  transition: all var(--transition);
}
.drawer-close:hover { color: var(--text-primary); background: var(--bg-hover); }

/* ===== topbar ===== */
.topbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}
.topbar-left > div {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.topbar-breadcrumb { font-size: 11.5px; color: var(--text-muted); }
.topbar-refresh {
  padding: 5px 8px;
  border-radius: 99px;
  color: var(--text-muted);
}
.topbar-refresh:hover { color: var(--accent); }
.docker-status {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 12px;
  border-radius: 99px;
  font-size: 12px;
  font-weight: 500;
}
.docker-status.ok {
  background: rgba(16,217,122,0.08);
  color: var(--green);
  border: 1px solid rgba(16,217,122,0.15);
}
.docker-status.err {
  background: rgba(240,84,100,0.08);
  color: var(--red);
  border: 1px solid rgba(240,84,100,0.15);
}
.status-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: currentColor;
  flex-shrink: 0;
}
.docker-status.ok .status-dot { animation: pulse-dot 2s infinite; }
.topbar-user {
  display: flex; align-items: center; gap: 8px;
  padding: 5px 12px;
  border-radius: 99px;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text-secondary);
  background: var(--bg-card);
  border: 1px solid var(--border);
  cursor: pointer;
}
.user-avatar {
  width: 22px; height: 22px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--cyan-600), var(--cyan-800));
  display: flex; align-items: center; justify-content: center;
  color: white;
}
.nav-badge {
  margin-left: auto;
  background: rgba(6,182,212,0.15);
  color: var(--accent);
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 99px;
  font-weight: 600;
}
.page-enter-active { transition: all 0.25s ease; }
.page-leave-active { transition: all 0.15s ease; }
.page-enter-from { opacity: 0; transform: translateY(6px); }
.page-leave-to   { opacity: 0; }

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  /* 侧边栏变为抽屉 */
  .sidebar {
    transform: translateX(-100%);
    transition: transform 0.28s cubic-bezier(0.4, 0, 0.2, 1);
    z-index: 200;
    box-shadow: none;
  }
  .sidebar.drawer-open {
    transform: translateX(0);
    box-shadow: 4px 0 32px rgba(0, 0, 0, 0.3);
  }

  /* 抽屉关闭按钮显示 */
  .drawer-close { display: flex; }

  /* 主内容撑满，不留侧边栏空间 */
  .main-content { margin-left: 0 !important; }

  /* 汉堡按钮显示 */
  .hamburger-btn { display: flex; }

  /* topbar 缩紧 */
  .topbar { padding: 0 16px !important; }

  /* 页面内容缩紧 */
  .page-content { padding: 16px !important; }

  /* 隐藏 Docker 文字，只保留圆点 */
  .docker-status-text { display: none; }
  .docker-status { padding: 6px 8px; border-radius: 50%; }

  /* 隐藏用户名 */
  .user-name { display: none; }
  .topbar-user { padding: 5px 7px; }
}
</style>

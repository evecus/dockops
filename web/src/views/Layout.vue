<template>
  <div class="app-layout">
    <!-- Sidebar -->
    <aside class="sidebar">
      <div class="sidebar-logo">
        <div class="logo-icon">
          <Container :size="18" color="white" />
        </div>
        <span class="logo-text">Dock<span>Ops</span></span>
      </div>

      <nav class="sidebar-nav">
        <RouterLink v-for="item in navItems" :key="item.path"
          :to="item.path" custom v-slot="{ isActive, navigate }">
          <div class="nav-item" :class="{ active: isActive }" @click="navigate">
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
          <div class="topbar-title">{{ currentTitle }}</div>
          <div class="topbar-breadcrumb" v-if="currentDesc">{{ currentDesc }}</div>
        </div>
        <div class="topbar-actions">
          <div class="docker-status" :class="dockerOk ? 'ok' : 'err'">
            <span class="status-dot"></span>
            <span>{{ dockerOk ? 'Docker 已连接' : 'Docker 未连接' }}</span>
          </div>
          <div class="topbar-user">
            <div class="user-avatar">
              <User :size="14" />
            </div>
            <span>Admin</span>
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
import { LayoutDashboard, Box, Image, Network, Settings, LogOut, Container, User } from 'lucide-vue-next'
import api from '@/api'

const router = useRouter()
const route = useRoute()
const dockerOk = ref(true)

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
.topbar-left { display: flex; flex-direction: column; gap: 2px; }
.topbar-breadcrumb { font-size: 11.5px; color: var(--text-muted); }
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
</style>

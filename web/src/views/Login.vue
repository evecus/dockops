<template>
  <div class="auth-page">
    <div class="auth-bg">
      <div class="auth-glow"></div>
      <div class="auth-grid"></div>
    </div>

    <div class="auth-card">
      <div class="auth-logo">
        <div class="logo-icon">
          <img src="/apple-touch-icon.png" width="28" height="28" style="border-radius:6px;display:block;" alt="DockOps" />
        </div>
        <h1>Dock<span>Ops</span></h1>
      </div>

      <p class="auth-subtitle">{{ isSetup ? '创建管理员账号' : '登录管理面板' }}</p>

      <div v-if="!checked" class="auth-loading">
        <div class="spinner"></div>
      </div>

      <form v-else @submit.prevent="submit" class="auth-form">
        <div class="form-group">
          <label class="form-label">用户名</label>
          <input v-model="form.username" class="form-input" placeholder="输入用户名"
            autocomplete="username" required />
        </div>
        <div class="form-group">
          <label class="form-label">密码</label>
          <div style="position:relative">
            <input v-model="form.password" class="form-input"
              :type="showPwd ? 'text' : 'password'"
              placeholder="输入密码" autocomplete="current-password" required />
            <button type="button" class="pwd-toggle" @click="showPwd = !showPwd">
              <Eye v-if="!showPwd" :size="15" />
              <EyeOff v-else :size="15" />
            </button>
          </div>
        </div>

        <div v-if="error" class="auth-error">
          <AlertCircle :size="14" /> {{ error }}
        </div>

        <button type="submit" class="btn btn-primary auth-submit" :disabled="loading">
          <div v-if="loading" class="spinner" style="width:14px;height:14px;border-width:2px"></div>
          <span>{{ isSetup ? '创建账号' : '登录' }}</span>
        </button>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Eye, EyeOff, AlertCircle } from 'lucide-vue-next'
import api from '@/api'

const router = useRouter()
const isSetup = ref(false)
const checked = ref(false)
const loading = ref(false)
const error = ref('')
const showPwd = ref(false)
const form = ref({ username: '', password: '' })

onMounted(async () => {
  try {
    const res = await api.systemStatus()
    const setup = res.data?.setup  // true = 已初始化, false = 未初始化

    if (!setup) {
      // 系统未初始化，需要创建管理员账号
      isSetup.value = true
    } else {
      // 系统已初始化，如果有 token 直接跳转 dashboard
      isSetup.value = false
      if (localStorage.getItem('token')) {
        router.replace('/dashboard')
        return
      }
    }
  } catch {}
  checked.value = true
})

async function submit() {
  error.value = ''
  loading.value = true
  try {
    if (isSetup.value) {
      await api.setup(form.value)
      const res = await api.login(form.value)
      localStorage.setItem('token', res.data.token)
    } else {
      const res = await api.login(form.value)
      localStorage.setItem('token', res.data.token)
    }
    router.replace('/dashboard')
  } catch (e) {
    error.value = typeof e === 'string' ? e : '操作失败，请重试'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
}
.auth-bg {
  position: fixed;
  inset: 0;
  z-index: 0;
}
.auth-glow {
  position: absolute;
  top: -200px;
  left: 50%;
  transform: translateX(-50%);
  width: 800px; height: 600px;
  background: radial-gradient(ellipse, rgba(6,182,212,0.12) 0%, transparent 65%);
}
.auth-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(6,182,212,0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(6,182,212,0.04) 1px, transparent 1px);
  background-size: 40px 40px;
}
.auth-card {
  position: relative;
  z-index: 1;
  background: var(--bg-card);
  border: 1px solid var(--border-2);
  border-radius: var(--radius-xl);
  padding: 40px;
  width: 380px;
  box-shadow: 0 32px 80px rgba(0,0,0,0.5), 0 0 60px rgba(6,182,212,0.06);
}
.auth-logo {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}
.auth-logo h1 {
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.5px;
}
.auth-logo h1 span { color: var(--accent); }
.auth-subtitle {
  font-size: 13px;
  color: var(--text-muted);
  margin-bottom: 28px;
}
.auth-loading {
  display: flex;
  justify-content: center;
  padding: 20px;
}
.auth-form { display: flex; flex-direction: column; gap: 16px; }
.pwd-toggle {
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  color: var(--text-muted);
  display: flex;
}
.pwd-toggle:hover { color: var(--text-secondary); }
.auth-error {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  background: rgba(240,84,100,0.08);
  border: 1px solid rgba(240,84,100,0.2);
  border-radius: var(--radius);
  font-size: 12.5px;
  color: var(--red);
}
.auth-submit {
  width: 100%;
  justify-content: center;
  padding: 10px;
  font-size: 14px;
  gap: 8px;
}
.auth-submit:disabled { opacity: 0.6; cursor: not-allowed; transform: none !important; }
</style>

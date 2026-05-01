<template>
  <div class="settings-page">
    <div class="settings-grid">

      <!-- Admin Account -->
      <div class="card">
        <div class="card-header">
          <div class="card-title"><ShieldCheck :size="16"/> 管理员账号</div>
        </div>
        <div class="card-body" style="display:flex;flex-direction:column;gap:14px">
          <div class="form-group">
            <label class="form-label">当前用户名</label>
            <div class="current-val">{{ adminUsername||'—' }}</div>
          </div>
          <div class="form-group">
            <label class="form-label">新用户名</label>
            <input v-model="admin.username" class="form-input" placeholder="留空则不修改"/>
          </div>
          <div class="form-group">
            <label class="form-label">新密码</label>
            <div style="position:relative">
              <input v-model="admin.password" class="form-input" :type="showPwd?'text':'password'" placeholder="留空则不修改"/>
              <button type="button" class="pwd-toggle" @click="showPwd=!showPwd">
                <Eye v-if="!showPwd" :size="14"/><EyeOff v-else :size="14"/>
              </button>
            </div>
          </div>
          <div class="form-group">
            <label class="form-label">确认密码</label>
            <input v-model="admin.confirm" class="form-input" :type="showPwd?'text':'password'" placeholder="再次输入新密码"/>
          </div>
          <button class="btn btn-primary" style="width:100%;justify-content:center" @click="saveAdmin" :disabled="savingAdmin">
            <div v-if="savingAdmin" class="spinner" style="width:13px;height:13px;border-width:2px"></div>
            <Save :size="14" v-else/> 保存账号信息
          </button>
        </div>
      </div>

      <!-- Update Check -->
      <div class="card">
        <div class="card-header"><div class="card-title"><Bell :size="16"/> 镜像更新检测</div></div>
        <div class="card-body" style="display:flex;flex-direction:column;gap:16px">
          <div class="form-group">
            <label class="form-label">检测频率</label>
            <select v-model="settings.update_check_interval" class="form-select">
              <option value="1h">每 1 小时</option>
              <option value="6h">每 6 小时（默认）</option>
              <option value="12h">每 12 小时</option>
              <option value="24h">每 24 小时</option>
            </select>
          </div>
          <div class="interval-hint">
            <Info :size="13" style="flex-shrink:0;color:var(--accent)"/>
            <span>定时检测镜像更新，检测到后在容器卡片上显示提示，支持一键更新</span>
          </div>
          <button class="btn btn-primary" style="width:100%;justify-content:center" @click="saveSettings" :disabled="savingSettings">
            <div v-if="savingSettings" class="spinner" style="width:13px;height:13px;border-width:2px"></div>
            <Save :size="14" v-else/> 保存设置
          </button>
        </div>
      </div>

      <!-- Docker Proxy -->
      <div class="card">
        <div class="card-header"><div class="card-title"><Globe :size="16"/> Docker 拉取代理</div></div>
        <div class="card-body" style="display:flex;flex-direction:column;gap:14px">
          <div class="form-group">
            <label class="form-label">镜像加速地址</label>
            <input v-model="settings.docker_proxy" class="form-input" placeholder="https://mirror.example.com"/>
          </div>
          <div class="proxy-example">
            <div class="example-title">常用镜像源（点击选择）</div>
            <div class="example-list">
              <div v-for="m in mirrors" :key="m.url" class="example-item" @click="settings.docker_proxy=m.url">
                <span class="example-name">{{ m.name }}</span>
                <span class="example-url">{{ m.url }}</span>
              </div>
            </div>
          </div>
          <button class="btn btn-primary" style="width:100%;justify-content:center" @click="saveSettings" :disabled="savingSettings">
            <Save :size="14"/> 保存设置
          </button>
        </div>
      </div>

      <!-- Install Docker -->
      <div class="card">
        <div class="card-header"><div class="card-title"><Terminal :size="16"/> 安装 Docker</div></div>
        <div class="card-body" style="display:flex;flex-direction:column;gap:14px">
          <p style="font-size:13px;color:var(--text-secondary);line-height:1.7">通过官方安装脚本一键安装 Docker，支持 Debian / Ubuntu / CentOS / Fedora 等主流 Linux 发行版，需要 root 权限。</p>
          <div class="install-cmd">
            <span class="install-prompt">$</span>
            <span class="install-code">curl -fsSL https://get.docker.com | sh</span>
            <button class="copy-btn" @click="copyInstallCmd"><Copy :size="13"/></button>
          </div>
          <div class="install-steps">
            <div class="step" v-for="(s,i) in installSteps" :key="i">
              <div class="step-num">{{ i+1 }}</div><span>{{ s }}</span>
            </div>
          </div>
          <button class="btn btn-ghost" style="width:100%;justify-content:center" @click="copyInstallCmd">
            <Copy :size="14"/> 复制安装命令
          </button>
        </div>
      </div>

      <!-- Docker Info -->
      <div class="card">
        <div class="card-header">
          <div class="card-title"><Info :size="16"/> Docker 连接信息</div>
          <button class="btn btn-ghost btn-sm" @click="loadDockerInfo"><RefreshCw :size="13"/></button>
        </div>
        <div class="card-body">
          <div v-if="!dockerInfo" class="empty-state" style="padding:20px 0"><div class="spinner"></div></div>
          <table v-else class="info-table">
            <tbody>
              <tr v-for="row in dockerInfoRows" :key="row.label">
                <td class="info-key">{{ row.label }}</td>
                <td class="info-val">{{ row.value }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- About -->
      <div class="card about-card">
        <div class="card-body" style="display:flex;flex-direction:column;align-items:center;gap:14px;padding:36px 24px">
          <div class="about-logo"><Container :size="28" color="white"/></div>
          <div style="text-align:center">
            <div class="about-name">DockOps</div>
            <div class="about-ver">v1.0.0</div>
            <div class="about-desc">基于 Docker Compose 的现代化容器管理平台</div>
          </div>
          <div class="about-tech">
            <span class="tech-badge">Go 1.22</span>
            <span class="tech-badge">Vue 3</span>
            <span class="tech-badge">SQLite</span>
            <span class="tech-badge">Docker SDK</span>
            <span class="tech-badge">WebSocket</span>
          </div>
        </div>
      </div>

    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ShieldCheck, Bell, Globe, Terminal, Info, RefreshCw, Save, Eye, EyeOff, Copy, Container } from 'lucide-vue-next'
import api from '@/api'
import { useToastStore } from '@/stores/toast'
const toast = useToastStore()
const showPwd = ref(false), savingAdmin = ref(false), savingSettings = ref(false)
const dockerInfo = ref(null), adminUsername = ref('')
const admin = ref({ username: '', password: '', confirm: '' })
const settings = ref({ update_check_interval: '6h', docker_proxy: '' })
const mirrors = [
  { name: '阿里云', url: 'https://registry.cn-hangzhou.aliyuncs.com' },
  { name: 'DaoCloud', url: 'https://hub-mirror.c.163.com' },
  { name: '腾讯云', url: 'https://mirror.ccs.tencentyun.com' },
  { name: 'USTC', url: 'https://docker.mirrors.ustc.edu.cn' },
]
const installSteps = ['确保系统已更新并具有 root 权限', '执行下方安装命令，等待完成', '运行 docker version 验证安装', '将当前用户加入 docker 组（可选）']
const dockerInfoRows = computed(() => {
  if (!dockerInfo.value) return []
  return [
    { label: 'Docker 版本', value: dockerInfo.value.docker_version || '—' },
    { label: '操作系统', value: dockerInfo.value.os || '—' },
    { label: '架构', value: dockerInfo.value.arch || '—' },
    { label: '内核版本', value: dockerInfo.value.kernel_version || '—' },
    { label: '存储驱动', value: dockerInfo.value.storage_driver || '—' },
    { label: '日志驱动', value: dockerInfo.value.logging_driver || '—' },
    { label: 'Docker 根目录', value: dockerInfo.value.docker_root_dir || '—' },
  ]
})
async function loadDockerInfo() { try { const r = await api.dashboardInfo(); dockerInfo.value = r.data } catch { dockerInfo.value = null } }
async function loadSettings() {
  try {
    const r = await api.getSettings(); const data = r.data || {}
    settings.value.update_check_interval = data.update_check_interval || '6h'
    settings.value.docker_proxy = data.docker_proxy || ''
    adminUsername.value = data.admin_username || 'admin'
  } catch {}
}
async function saveAdmin() {
  if (!admin.value.username && !admin.value.password) { toast.error('请填写要修改的内容'); return }
  if (admin.value.password && admin.value.password !== admin.value.confirm) { toast.error('两次密码不一致'); return }
  savingAdmin.value = true
  try {
    await api.updateAdmin({ username: admin.value.username || adminUsername.value, password: admin.value.password || 'keep' })
    toast.success('账号已更新，请重新登录')
    setTimeout(() => { localStorage.removeItem('token'); location.href = '/login' }, 1500)
  } catch (e) { toast.error(typeof e === 'string' ? e : '保存失败') } finally { savingAdmin.value = false }
}
async function saveSettings() {
  savingSettings.value = true
  try { await api.updateSettings({ update_check_interval: settings.value.update_check_interval, docker_proxy: settings.value.docker_proxy }); toast.success('设置已保存') }
  catch (e) { toast.error(typeof e === 'string' ? e : '保存失败') } finally { savingSettings.value = false }
}
function copyInstallCmd() {
  navigator.clipboard.writeText('curl -fsSL https://get.docker.com | sh')
    .then(() => toast.success('命令已复制到剪贴板')).catch(() => toast.error('复制失败'))
}
onMounted(() => { loadSettings(); loadDockerInfo() })
</script>

<style scoped>
.settings-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(380px,1fr));gap:20px}
.current-val{font-family:var(--font-mono);font-size:13px;color:var(--accent-light);padding:8px 12px;background:var(--bg-input);border-radius:var(--radius);border:1px solid var(--border)}
.pwd-toggle{position:absolute;right:10px;top:50%;transform:translateY(-50%);background:none;color:var(--text-muted);display:flex}
.pwd-toggle:hover{color:var(--text-secondary)}
.interval-hint{display:flex;align-items:flex-start;gap:8px;padding:10px 12px;background:rgba(6,182,212,0.05);border:1px solid rgba(6,182,212,0.12);border-radius:var(--radius);font-size:12px;color:var(--text-muted);line-height:1.5}
.proxy-example{background:var(--bg-input);border:1px solid var(--border);border-radius:var(--radius);overflow:hidden}
.example-title{padding:7px 12px;font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.6px;color:var(--text-muted);border-bottom:1px solid var(--border)}
.example-list{display:flex;flex-direction:column}
.example-item{display:flex;align-items:center;justify-content:space-between;padding:8px 12px;cursor:pointer;transition:background var(--transition);gap:12px}
.example-item:hover{background:rgba(6,182,212,0.05)}
.example-name{font-size:12.5px;font-weight:500;color:var(--text-secondary);flex-shrink:0}
.example-url{font-size:11px;font-family:var(--font-mono);color:var(--text-muted);word-break:break-all}
.install-cmd{display:flex;align-items:center;gap:8px;background:var(--bg-base);border:1px solid var(--border);border-radius:var(--radius);padding:10px 14px}
.install-prompt{color:var(--green);font-family:var(--font-mono);font-size:13px;flex-shrink:0}
.install-code{font-family:var(--font-mono);font-size:12.5px;color:var(--text-code);flex:1}
.copy-btn{background:transparent;color:var(--text-muted);display:flex;padding:3px;border-radius:4px;cursor:pointer}
.copy-btn:hover{color:var(--accent)}
.install-steps{display:flex;flex-direction:column;gap:6px}
.step{display:flex;align-items:center;gap:10px;font-size:12.5px;color:var(--text-muted)}
.step-num{width:20px;height:20px;border-radius:50%;background:rgba(6,182,212,0.1);border:1px solid var(--border-2);display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:700;color:var(--accent);flex-shrink:0}
.info-table{width:100%}
.info-key{font-size:12px;color:var(--text-muted);padding:5px 0;width:110px;font-weight:500}
.info-val{font-size:12.5px;font-family:var(--font-mono);color:var(--text-secondary);padding:5px 0}
.about-card{}
.about-logo{width:60px;height:60px;background:linear-gradient(135deg,var(--cyan-500),var(--cyan-700));border-radius:16px;display:flex;align-items:center;justify-content:center;box-shadow:0 0 30px rgba(6,182,212,0.3)}
.about-name{font-size:22px;font-weight:800;letter-spacing:-0.5px;margin-bottom:2px}
.about-ver{font-size:12px;color:var(--accent);font-family:var(--font-mono);margin-bottom:8px}
.about-desc{font-size:13px;color:var(--text-muted);line-height:1.5}
.about-tech{display:flex;flex-wrap:wrap;gap:6px;justify-content:center}
.tech-badge{padding:3px 10px;border-radius:99px;font-size:11px;font-weight:600;background:rgba(6,182,212,0.08);color:var(--accent);border:1px solid rgba(6,182,212,0.15)}
</style>

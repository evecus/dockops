<template>
  <div class="dashboard">
    <!-- Stats Row -->
    <div class="grid-4 mb-6">
      <div class="stat-card" v-for="stat in statCards" :key="stat.label">
        <div class="stat-icon" :style="`background:${stat.bg}`">
          <component :is="stat.icon" :size="18" :style="`color:${stat.color}`" />
        </div>
        <div class="stat-value" :style="`color:${stat.color}`">{{ stat.value }}</div>
        <div class="stat-label">{{ stat.label }}</div>
      </div>
    </div>

    <div class="grid-2 mb-6">
      <!-- System Info -->
      <div class="card">
        <div class="card-header">
          <div class="card-title"><Server :size="16" /> 主机信息</div>
          <span class="badge badge-cyan">{{ info?.docker_version || '—' }}</span>
        </div>
        <div class="card-body">
          <div v-if="loading" class="empty-state"><div class="spinner"></div></div>
          <table v-else class="info-table">
            <tbody>
              <tr v-for="row in infoRows" :key="row.label">
                <td class="info-key">{{ row.label }}</td>
                <td class="info-val">{{ row.value }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Resource Usage -->
      <div class="card">
        <div class="card-header">
          <div class="card-title"><Activity :size="16" /> 资源占用</div>
          <span v-if="cacheTime" class="cache-hint">{{ cacheTimeText }}</span>
        </div>
        <div class="card-body">
          <div class="resource-gauges">
            <div class="gauge-item">
              <div class="gauge-header">
                <span class="gauge-label">CPU 占用</span>
                <span class="gauge-val">{{ stats?.total_cpu_percent?.toFixed(1) || '0.0' }}%</span>
              </div>
              <div class="progress-bar">
                <div class="progress-fill" :style="`width:${Math.min(stats?.total_cpu_percent || 0, 100)}%`"></div>
              </div>
            </div>

            <div class="gauge-item">
              <div class="gauge-header">
                <span class="gauge-label">内存占用</span>
                <span class="gauge-val">{{ formatBytes(stats?.total_mem_usage) }} / {{ formatBytes(info?.total_memory) }}</span>
              </div>
              <div class="progress-bar">
                <div class="progress-fill" :style="`width:${memPercent}%`"
                  :class="memPercent > 80 ? 'fill-red' : memPercent > 60 ? 'fill-amber' : ''"></div>
              </div>
            </div>
          </div>

          <!-- Container breakdown -->
          <div class="container-breakdown">
            <div class="breakdown-item" v-for="b in breakdown" :key="b.label">
              <div class="breakdown-dot" :style="`background:${b.color}`"></div>
              <span class="breakdown-label">{{ b.label }}</span>
              <span class="breakdown-val">{{ b.value }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import {
  Server, Activity,
  Cpu, MemoryStick, HardDrive, Container, Box
} from 'lucide-vue-next'
import api from '@/api'

const info = ref(null)
const stats = ref(null)
const loading = ref(true)
const cacheTime = ref(null)

const cacheTimeText = computed(() => {
  if (!cacheTime.value) return ''
  const diff = Math.floor((Date.now() - cacheTime.value) / 1000)
  if (diff < 60) return `${diff}s 前更新`
  if (diff < 3600) return `${Math.floor(diff / 60)}m 前更新`
  return `${Math.floor(diff / 3600)}h 前更新`
})

const memPercent = computed(() => {
  if (!stats.value?.total_mem_usage || !info.value?.total_memory) return 0
  return Math.min(stats.value.total_mem_usage / info.value.total_memory * 100, 100)
})

const statCards = computed(() => [
  { label: '运行中容器', value: info.value?.containers_running ?? '—', icon: Box, color: 'var(--green)', bg: 'rgba(16,217,122,0.1)' },
  { label: '全部容器', value: info.value?.containers ?? '—', icon: Container, color: 'var(--accent)', bg: 'rgba(6,182,212,0.1)' },
  { label: '本地镜像', value: info.value?.images ?? '—', icon: HardDrive, color: 'var(--purple)', bg: 'rgba(167,139,250,0.1)' },
  { label: 'CPU 核心', value: info.value?.cpus ?? '—', icon: Cpu, color: 'var(--amber)', bg: 'rgba(245,158,11,0.1)' },
])

const infoRows = computed(() => [
  { label: 'Docker 版本', value: info.value?.docker_version || '—' },
  { label: '操作系统', value: info.value?.os || '—' },
  { label: '系统架构', value: info.value?.arch || '—' },
  { label: '内核版本', value: info.value?.kernel_version || '—' },
  { label: '存储驱动', value: info.value?.storage_driver || '—' },
  { label: 'Docker 根目录', value: info.value?.docker_root_dir || '—' },
  { label: '总内存', value: formatBytes(info.value?.total_memory) },
  { label: '系统时间', value: info.value?.server_time ? new Date(info.value.server_time).toLocaleString('zh-CN') : '—' },
])

const breakdown = computed(() => [
  { label: '运行中', value: info.value?.containers_running ?? 0, color: 'var(--green)' },
  { label: '已停止', value: info.value?.containers_stopped ?? 0, color: 'var(--text-muted)' },
  { label: '已暂停', value: info.value?.containers_paused ?? 0, color: 'var(--amber)' },
])

function formatBytes(b) {
  if (!b) return '—'
  if (b >= 1e9) return (b / 1e9).toFixed(1) + ' GB'
  if (b >= 1e6) return (b / 1e6).toFixed(1) + ' MB'
  return (b / 1e3).toFixed(0) + ' KB'
}

async function loadCached() {
  try {
    const [i, s] = await Promise.all([api.dashboardInfo(), api.dashboardStats()])
    info.value = i.data
    stats.value = s.data
    cacheTime.value = Date.now()
  } catch {}
}

onMounted(async () => {
  try { await loadCached() }
  finally { loading.value = false }
})
</script>

<style scoped>
.dashboard {}
.mb-6 { margin-bottom: 24px; }
.info-table { width: 100%; }
.info-key {
  font-size: 12px;
  color: var(--text-muted);
  padding: 6px 0;
  width: 120px;
  font-weight: 500;
}
.info-val {
  font-size: 12.5px;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  padding: 6px 0;
}
.resource-gauges { display: flex; flex-direction: column; gap: 20px; margin-bottom: 24px; }
.gauge-item { display: flex; flex-direction: column; gap: 8px; }
.gauge-header { display: flex; justify-content: space-between; align-items: center; }
.gauge-label { font-size: 12.5px; color: var(--text-secondary); font-weight: 500; }
.gauge-val { font-size: 13px; font-family: var(--font-mono); color: var(--accent-light); }
.fill-red { background: linear-gradient(90deg, var(--red), #ff6b6b) !important; }
.fill-amber { background: linear-gradient(90deg, var(--amber), #fcd34d) !important; }
.container-breakdown {
  display: flex;
  gap: 20px;
  padding: 14px 16px;
  background: var(--bg-input);
  border-radius: var(--radius);
  border: 1px solid var(--border);
}
.breakdown-item { display: flex; align-items: center; gap: 7px; }
.breakdown-dot { width: 8px; height: 8px; border-radius: 50%; }
.breakdown-label { font-size: 12px; color: var(--text-muted); }
.breakdown-val { font-size: 13px; font-weight: 600; color: var(--text-primary); }
.cache-hint {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}
</style>

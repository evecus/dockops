<template>
  <div class="containers-page">
    <!-- Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="filter-group">
          <button class="filter-btn" :class="filter === 'all' ? 'active' : ''" @click="filter='all'">全部 <span class="filter-count">{{ containers.length }}</span></button>
          <button class="filter-btn" :class="filter === 'running' ? 'active' : ''" @click="filter='running'">运行中 <span class="filter-count">{{ runningCount }}</span></button>
          <button class="filter-btn" :class="filter === 'stopped' ? 'active' : ''" @click="filter='stopped'">已停止</button>
        </div>
      </div>
      <div class="header-right">
        <button class="btn btn-ghost" @click="checkUpdates" :disabled="checking">
          <RefreshCw :size="14" :class="checking ? 'spin' : ''" /> 检查更新
        </button>
        <button class="btn btn-primary" @click="showCreate = true">
          <Plus :size="15" /> 创建容器
        </button>
      </div>
    </div>

    <!-- Grid -->
    <div v-if="loading" class="empty-state"><div class="spinner"></div></div>
    <div v-else-if="!filtered.length" class="empty-state">
      <Box :size="48" />
      <p>暂无容器</p>
      <button class="btn btn-primary" @click="showCreate = true"><Plus :size="14" /> 创建第一个容器</button>
    </div>
    <div v-else class="container-grid">
      <div v-for="ct in filtered" :key="ct.id" class="ct-card" @click="openDetail(ct)">
        <!-- Header -->
        <div class="ct-card-header">
          <div class="ct-name">
            <div class="ct-indicator" :class="stateClass(ct.docker_state)"></div>
            <span>{{ ct.name }}</span>
          </div>
          <div class="ct-badges">
            <span v-if="ct.update_available" class="badge badge-amber" @click.stop="updateImage(ct)">
              <ArrowUp :size="10" /> 有更新
            </span>
            <span class="badge" :class="badgeClass(ct.docker_state)">
              {{ stateLabel(ct.docker_state) }}
            </span>
            <span v-if="ct.docker_status" class="ct-status-text">{{ ct.docker_status }}</span>
          </div>
        </div>

        <!-- Info -->
        <div class="ct-info">
          <div class="ct-info-row" v-if="ct.source === 'external'">
            <span class="ct-info-key">来源</span>
            <span class="ct-info-val tag" style="color:var(--amber);background:rgba(245,158,11,0.1);border-color:rgba(245,158,11,0.2)">外部容器</span>
          </div>
          <div class="ct-info-row" v-if="ct.source !== 'external'">
            <span class="ct-info-key">目录</span>
            <span class="ct-info-val tag">{{ shortPath(ct.compose_dir) }}</span>
          </div>
          <div class="ct-info-row" v-if="ct.ports?.length">
            <span class="ct-info-key">端口</span>
            <div class="ct-ports">
              <span class="tag" v-for="p in ct.ports.slice(0,3)" :key="p.host_port">
                {{ p.host_port }}→{{ p.container_port }}
              </span>
              <span v-if="ct.ports.length > 3" class="sep">+{{ ct.ports.length-3 }}</span>
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div class="ct-actions" @click.stop>
          <button class="btn btn-icon" v-if="ct.docker_state !== 'running'" @click="start(ct)" data-tip="启动">
            <Play :size="13" />
          </button>
          <button class="btn btn-icon" v-else @click="stop(ct)" data-tip="停止">
            <Square :size="13" />
          </button>
          <button class="btn btn-icon" @click="restart(ct)" data-tip="重启">
            <RotateCcw :size="13" />
          </button>
          <button class="btn btn-icon" @click="openTerminal(ct)" data-tip="终端">
            <Terminal :size="13" />
          </button>
          <button class="btn btn-icon" @click="openLogs(ct)" data-tip="日志">
            <ScrollText :size="13" />
          </button>
          <button class="btn btn-icon" @click="openFiles(ct)" data-tip="文件">
            <FolderOpen :size="13" />
          </button>
          <button class="btn btn-icon" @click="editContainer(ct)" data-tip="编辑">
            <Pencil :size="13" />
          </button>
          <button class="btn btn-icon btn-danger-icon" @click="confirmDelete(ct)" data-tip="删除">
            <Trash2 :size="13" />
          </button>
        </div>
      </div>
    </div>

    <!-- Create Modal -->
    <Teleport to="body">
      <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
        <CreateContainerModal @close="showCreate = false" @created="onCreated" />
      </div>

      <!-- Detail / Edit Modal -->
      <div v-if="detailCt" class="modal-overlay" @click.self="detailCt = null">
        <ContainerDetailModal :container="detailCt" @close="detailCt = null"
          @edit="editContainer(detailCt)" @refresh="load" />
      </div>

      <!-- Edit Modal -->
      <div v-if="editCt" class="modal-overlay" @click.self="editCt = null">
        <EditContainerModal :container="editCt" @close="editCt = null" @saved="onCreated" />
      </div>

      <!-- Terminal Modal -->
      <div v-if="termCt" class="modal-overlay" @click.self="termCt = null">
        <TerminalModal :container="termCt" @close="termCt = null" />
      </div>

      <!-- Logs Modal -->
      <div v-if="logsCt" class="modal-overlay" @click.self="logsCt = null">
        <LogsModal :container="logsCt" @close="logsCt = null" />
      </div>

      <!-- Files Modal -->
      <div v-if="filesCt" class="modal-overlay" @click.self="filesCt = null">
        <FilesModal :container="filesCt" @close="filesCt = null" />
      </div>

      <!-- Delete Confirm -->
      <div v-if="deleteCt" class="modal-overlay" @click.self="deleteCt = null">
        <div class="modal" style="max-width:420px">
          <div class="modal-header">
            <div class="modal-title"><Trash2 :size="16" /> 删除容器</div>
            <button class="modal-close" @click="deleteCt = null"><X :size="15" /></button>
          </div>
          <div class="modal-body">
            <p style="color:var(--text-secondary);font-size:14px">
              确定要删除容器 <strong style="color:var(--text-primary)">{{ deleteCt.name }}</strong> 吗？
              这将停止并移除容器及其 Compose 配置。
            </p>
          </div>
          <div class="modal-footer">
            <button class="btn btn-ghost" @click="deleteCt = null">取消</button>
            <button class="btn btn-danger" @click="doDelete" :disabled="deleting">
              <div v-if="deleting" class="spinner" style="width:12px;height:12px;border-width:2px"></div>
              确认删除
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import {
  Plus, RefreshCw, Box, Play, Square, RotateCcw, Terminal,
  ScrollText, FolderOpen, Pencil, Trash2, X, ArrowUp
} from 'lucide-vue-next'
import api from '@/api'
import { useToastStore } from '@/stores/toast'
import CreateContainerModal from '@/components/CreateContainerModal.vue'
import ContainerDetailModal from '@/components/ContainerDetailModal.vue'
import EditContainerModal from '@/components/EditContainerModal.vue'
import TerminalModal from '@/components/TerminalModal.vue'
import LogsModal from '@/components/LogsModal.vue'
import FilesModal from '@/components/FilesModal.vue'

const toast = useToastStore()
const containers = ref([])
const loading = ref(true)
const filter = ref('all')
const checking = ref(false)
const showCreate = ref(false)
const detailCt = ref(null)
const editCt = ref(null)
const termCt = ref(null)
const logsCt = ref(null)
const filesCt = ref(null)
const deleteCt = ref(null)
const deleting = ref(false)

const runningCount = computed(() => containers.value.filter(c => c.docker_state === 'running').length)
const filtered = computed(() => {
  if (filter.value === 'running') return containers.value.filter(c => c.docker_state === 'running')
  if (filter.value === 'stopped') return containers.value.filter(c => c.docker_state !== 'running')
  return containers.value
})

function stateClass(s) {
  if (s === 'running') return 'running'
  if (s === 'exited' || s === 'dead') return 'stopped'
  return 'paused'
}
function badgeClass(s) {
  if (s === 'running') return 'badge-green badge-dot'
  if (s === 'exited' || s === 'dead') return 'badge-red'
  return 'badge-muted'
}
function stateLabel(s) {
  return { running: '运行中', exited: '已退出', paused: '已暂停', dead: '异常', created: '已创建' }[s] || s || '未知'
}
function shortPath(p) {
  if (!p) return '—'
  const parts = p.split('/')
  return parts.length > 3 ? '…/' + parts.slice(-2).join('/') : p
}

async function load() {
  try {
    const res = await api.listContainers()
    containers.value = res.data || []
  } catch (e) { toast.error('加载容器失败') }
  finally { loading.value = false }
}

async function start(ct) {
  try { await api.startContainer(ct.id); toast.success('已启动'); load() }
  catch (e) { toast.error(e) }
}
async function stop(ct) {
  try { await api.stopContainer(ct.id); toast.success('已停止'); load() }
  catch (e) { toast.error(e) }
}
async function restart(ct) {
  try { await api.restartContainer(ct.id); toast.success('已重启'); load() }
  catch (e) { toast.error(e) }
}
async function updateImage(ct) {
  try { await api.updateContainerImage(ct.id); toast.success('更新完成'); load() }
  catch (e) { toast.error(e) }
}
async function checkUpdates() {
  checking.value = true
  try { await api.checkUpdates(); toast.info('已触发更新检查') }
  catch (e) { toast.error(e) }
  finally { checking.value = false; setTimeout(load, 3000) }
}

function openDetail(ct) { detailCt.value = ct }
function openTerminal(ct) { termCt.value = ct }
function openLogs(ct) { logsCt.value = ct }
function openFiles(ct) { filesCt.value = ct }
function editContainer(ct) { editCt.value = ct; detailCt.value = null }
function confirmDelete(ct) { deleteCt.value = ct }

async function doDelete() {
  if (!deleteCt.value) return
  deleting.value = true
  try {
    await api.deleteContainer(deleteCt.value.id)
    toast.success('容器已删除')
    deleteCt.value = null
    load()
  } catch (e) { toast.error(e) }
  finally { deleting.value = false }
}

function onCreated() {
  showCreate.value = false
  editCt.value = null
  load()
}

onMounted(load)
</script>

<style scoped>
.containers-page {}
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}
.header-left, .header-right { display: flex; align-items: center; gap: 10px; }
.filter-group {
  display: flex;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 3px;
  gap: 2px;
}
.filter-btn {
  padding: 5px 12px;
  border-radius: calc(var(--radius) - 3px);
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text-muted);
  background: none;
  display: flex; align-items: center; gap: 6px;
  transition: all var(--transition);
  cursor: pointer;
}
.filter-btn:hover { color: var(--text-secondary); }
.filter-btn.active { background: var(--accent-dim); color: var(--accent-light); }
.filter-count {
  background: rgba(6,182,212,0.15);
  color: var(--accent);
  font-size: 10px;
  padding: 0 5px;
  border-radius: 99px;
}
.spin { animation: spin 0.8s linear infinite; }

.container-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(310px, 1fr));
  gap: 16px;
}
.ct-card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 16px;
  cursor: pointer;
  transition: all var(--transition);
  display: flex;
  flex-direction: column;
  gap: 12px;
  position: relative;
  overflow: hidden;
}
.ct-card::before {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--accent), transparent);
  opacity: 0;
  transition: opacity var(--transition);
}
.ct-card:hover { border-color: var(--border-2); box-shadow: var(--shadow-cyan); transform: translateY(-2px); }
.ct-card:hover::before { opacity: 0.6; }

.ct-card-header { display: flex; align-items: center; justify-content: space-between; }
.ct-name { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 14px; }
.ct-indicator {
  width: 8px; height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.ct-indicator.running { background: var(--green); box-shadow: 0 0 8px var(--green); animation: pulse-dot 2s infinite; }
.ct-indicator.stopped { background: var(--text-muted); }
.ct-indicator.paused  { background: var(--amber); }
.ct-badges { display: flex; align-items: center; gap: 5px; }

.ct-info { display: flex; flex-direction: column; gap: 6px; }
.ct-info-row { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.ct-info-key { color: var(--text-muted); min-width: 28px; font-weight: 500; }
.ct-info-val { color: var(--text-secondary); font-family: var(--font-mono); font-size: 11px; }
.ct-ports { display: flex; flex-wrap: wrap; gap: 4px; }

.ct-actions {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}
.btn-danger-icon { color: var(--red) !important; }
.btn-danger-icon:hover { background: rgba(240,84,100,0.1) !important; border-color: rgba(240,84,100,0.25) !important; }
.ct-status-text {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
  white-space: nowrap;
}
</style>

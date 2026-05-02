<template>
  <div class="modal modal-xl">
    <div class="modal-header">
      <div class="modal-title">
        <component :is="editing ? Pencil : Plus" :size="16" />
        {{ editing ? '编辑容器' : '创建容器' }}
      </div>
      <button class="modal-close" @click="$emit('close')"><X :size="15" /></button>
    </div>

    <div class="modal-body" style="padding:0">
      <div class="create-layout">
        <!-- Left: basic info + mode selector -->
        <div class="create-sidebar">
          <div class="sidebar-section">
            <div class="form-group">
              <label class="form-label">容器名称 *</label>
              <input v-model="form.name" class="form-input" placeholder="my-app" required />
            </div>
          </div>

          <div class="sidebar-section">
            <div class="form-label" style="margin-bottom:8px">创建方式</div>
            <div class="mode-list">
              <div v-for="m in modes" :key="m.id"
                class="mode-item" :class="{ active: mode === m.id }"
                @click="mode = m.id">
                <component :is="m.icon" :size="15" />
                <div>
                  <div class="mode-name">{{ m.label }}</div>
                  <div class="mode-desc">{{ m.desc }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Right: content based on mode -->
        <div class="create-main">
          <!-- Upload mode -->
          <div v-if="mode === 'upload'" class="mode-panel">
            <div class="upload-zone" :class="{ dragging }"
              @dragover.prevent="dragging=true" @dragleave="dragging=false"
              @drop.prevent="onDrop">
              <Upload :size="32" style="opacity:0.4;margin-bottom:12px" />
              <p style="font-size:14px;color:var(--text-secondary)">拖拽 YAML 文件到此处</p>
              <p style="font-size:12px;color:var(--text-muted);margin-top:4px">或</p>
              <label class="btn btn-ghost" style="margin-top:8px;cursor:pointer">
                选择文件
                <input type="file" accept=".yml,.yaml" @change="onFileSelect" style="display:none" />
              </label>
            </div>
            <div v-if="form.compose_content" class="compose-preview">
              <div class="compose-preview-header">
                <span>预览</span>
                <button class="btn btn-ghost btn-sm" @click="form.compose_content=''">清空</button>
              </div>
              <pre class="code-block" style="max-height:300px">{{ form.compose_content }}</pre>
            </div>
          </div>

          <!-- Paste mode -->
          <div v-if="mode === 'paste'" class="mode-panel">
            <label class="form-label">粘贴 Compose 内容</label>
            <textarea v-model="form.compose_content" class="form-textarea"
              style="min-height:400px;font-size:12.5px"
              placeholder="version: '3.8'
services:
  app:
    image: nginx:latest
    ports:
      - '80:80'
    restart: unless-stopped"></textarea>
          </div>

          <!-- Docker Run mode -->
          <div v-if="mode === 'run'" class="mode-panel">
            <div class="form-group">
              <label class="form-label">粘贴 docker run 命令</label>
              <textarea v-model="runCmd" class="form-textarea" style="min-height:100px;font-size:12.5px"
                placeholder="docker run -d --name nginx -p 80:80 -v /data:/data --restart unless-stopped nginx:latest"></textarea>
              <button class="btn btn-primary" style="width:100%;justify-content:center;margin-top:8px" @click="parseRun" :disabled="parsing">
                <component :is="parsing ? RefreshCw : Wand2" :size="14" :class="parsing?'spin':''" />
                {{ parsing ? '解析中...' : '解析命令' }}
              </button>
            </div>
            <div v-if="parsedYaml" class="compose-preview">
              <div class="compose-preview-header">
                <span>生成的 Compose 配置（可编辑）</span>
                <button class="btn btn-ghost btn-sm" @click="parsedYaml=''">重新解析</button>
              </div>
              <textarea v-model="parsedYaml" class="form-textarea" style="min-height:280px;font-size:12.5px"
                @input="form.compose_content = parsedYaml"></textarea>
            </div>
          </div>

          <!-- Form mode -->
          <div v-if="mode === 'form'" class="mode-panel">
            <div class="form-tabs">
              <div class="tabs">
                <div v-for="t in formTabs" :key="t.id" class="tab" :class="{ active: formTab === t.id }" @click="formTab = t.id">
                  {{ t.label }}
                </div>
              </div>
            </div>

            <!-- Basic tab -->
            <div v-if="formTab === 'basic'" class="tab-content">
              <div class="grid-2">
                <div class="form-group">
                  <label class="form-label">镜像 *</label>
                  <input v-model="formFields.image" class="form-input" placeholder="nginx:latest" />
                </div>
                <div class="form-group">
                  <label class="form-label">重启策略</label>
                  <select v-model="formFields.restart" class="form-select">
                    <option value="">不重启</option>
                    <option value="always">always</option>
                    <option value="unless-stopped">unless-stopped</option>
                    <option value="on-failure">on-failure</option>
                  </select>
                </div>
              </div>
              <div class="form-group">
                <label class="form-label">Hostname</label>
                <input v-model="formFields.hostname" class="form-input" placeholder="可选" />
              </div>
              <div class="form-group" style="margin-top:4px">
                <label class="form-label" style="display:flex;align-items:center;gap:6px">
                  <input type="checkbox" v-model="formFields.privileged" />
                  Privileged 模式
                </label>
              </div>
            </div>

            <!-- Ports tab -->
            <div v-if="formTab === 'ports'" class="tab-content">
              <div class="list-editor">
                <div v-for="(p, i) in formFields.ports" :key="i" class="list-row">
                  <input v-model="formFields.ports[i]" class="form-input" placeholder="8080:80/tcp" />
                  <button class="btn btn-icon" @click="formFields.ports.splice(i,1)"><X :size="13"/></button>
                </div>
                <button class="btn btn-ghost btn-sm" @click="formFields.ports.push('')"><Plus :size="13"/> 添加端口</button>
              </div>
            </div>

            <!-- Volumes tab -->
            <div v-if="formTab === 'volumes'" class="tab-content">
              <div class="list-editor">
                <div v-for="(v, i) in formFields.volumes" :key="i" class="list-row">
                  <input v-model="formFields.volumes[i]" class="form-input" placeholder="/host/path:/container/path" />
                  <button class="btn btn-icon" @click="formFields.volumes.splice(i,1)"><X :size="13"/></button>
                </div>
                <button class="btn btn-ghost btn-sm" @click="formFields.volumes.push('')"><Plus :size="13"/> 添加挂载</button>
              </div>
            </div>

            <!-- Env tab -->
            <div v-if="formTab === 'env'" class="tab-content">
              <div class="list-editor">
                <div v-for="(e, i) in formFields.env" :key="i" class="list-row">
                  <input v-model="formFields.env[i]" class="form-input" placeholder="KEY=VALUE" />
                  <button class="btn btn-icon" @click="formFields.env.splice(i,1)"><X :size="13"/></button>
                </div>
                <button class="btn btn-ghost btn-sm" @click="formFields.env.push('')"><Plus :size="13"/> 添加变量</button>
              </div>
            </div>

            <!-- Command tab -->
            <div v-if="formTab === 'cmd'" class="tab-content">
              <div class="form-group">
                <label class="form-label">启动命令</label>
                <input v-model="formFields.command" class="form-input" placeholder="可选，覆盖默认 CMD" />
              </div>
              <div class="form-group">
                <label class="form-label">Entrypoint</label>
                <input v-model="formFields.entrypoint" class="form-input" placeholder="可选" />
              </div>
              <div class="form-group">
                <label class="form-label">User</label>
                <input v-model="formFields.user" class="form-input" placeholder="可选，例如 1000:1000" />
              </div>
              <div class="form-group">
                <label class="form-label">Network Mode</label>
                <select v-model="formFields.network_mode" class="form-select">
                  <option value="">默认 (bridge)</option>
                  <option value="host">host</option>
                  <option value="none">none</option>
                </select>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Generated YAML preview for form mode -->
    <div v-if="mode === 'form' && generatedYaml" class="yaml-preview-bar">
      <span class="form-label" style="flex-shrink:0">生成的 Compose:</span>
      <pre class="yaml-preview-code">{{ generatedYaml }}</pre>
    </div>

    <div class="modal-footer">
      <button class="btn btn-ghost" @click="$emit('close')">取消</button>
      <button class="btn btn-primary" @click="submit" :disabled="submitting">
        <div v-if="submitting" class="spinner" style="width:13px;height:13px;border-width:2px"></div>
        {{ editing ? '保存并重启' : '创建并启动' }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { X, Plus, Upload, Wand2, RefreshCw, Pencil, FileText, ClipboardPaste, Terminal, Settings2 } from 'lucide-vue-next'
import api from '@/api'
import { useToastStore } from '@/stores/toast'

const props = defineProps({ editing: Object })
const emit = defineEmits(['close', 'created'])
const toast = useToastStore()

const mode = ref('paste')
const runCmd = ref('')
const parsedYaml = ref('')
const parsing = ref(false)
const submitting = ref(false)
const dragging = ref(false)
const formTab = ref('basic')

const form = ref({
  name: '',
  create_mode: 'paste',
  compose_content: ''
})

const formFields = ref({
  image: '', restart: 'unless-stopped', hostname: '', privileged: false,
  ports: [], volumes: [], env: [], command: '', entrypoint: '', user: '', network_mode: ''
})

const modes = [
  { id: 'upload', label: '上传文件', desc: '上传 YAML 文件', icon: Upload },
  { id: 'paste', label: '粘贴内容', desc: '直接粘贴 Compose', icon: ClipboardPaste },
  { id: 'run', label: 'Docker Run', desc: '解析 docker run 命令', icon: Terminal },
  { id: 'form', label: '表单填写', desc: '图形化配置', icon: Settings2 },
]

const formTabs = [
  { id: 'basic', label: '基础配置' },
  { id: 'ports', label: '端口映射' },
  { id: 'volumes', label: '卷挂载' },
  { id: 'env', label: '环境变量' },
  { id: 'cmd', label: '高级配置' },
]

const generatedYaml = computed(() => {
  if (mode.value !== 'form' || !formFields.value.image) return ''
  const f = formFields.value
  const name = form.value.name || f.image.split('/').pop().split(':')[0] || 'app'
  let y = `version: '3.8'\n\nservices:\n  ${name}:\n    image: ${f.image}\n`
  if (f.restart) y += `    restart: ${f.restart}\n`
  if (f.hostname) y += `    hostname: ${f.hostname}\n`
  if (f.privileged) y += `    privileged: true\n`
  if (f.network_mode) y += `    network_mode: ${f.network_mode}\n`
  if (f.command) y += `    command: ${f.command}\n`
  if (f.entrypoint) y += `    entrypoint: ${f.entrypoint}\n`
  if (f.user) y += `    user: "${f.user}"\n`
  const ports = f.ports.filter(Boolean)
  if (ports.length) { y += '    ports:\n'; ports.forEach(p => y += `      - "${p}"\n`) }
  const vols = f.volumes.filter(Boolean)
  if (vols.length) { y += '    volumes:\n'; vols.forEach(v => y += `      - ${v}\n`) }
  const envs = f.env.filter(Boolean)
  if (envs.length) { y += '    environment:\n'; envs.forEach(e => y += `      - ${e}\n`) }
  return y
})

watch(generatedYaml, val => { if (mode.value === 'form') form.value.compose_content = val })
watch(mode, m => { form.value.create_mode = m })

function onDrop(e) {
  dragging.value = false
  const file = e.dataTransfer.files[0]
  if (file) readFile(file)
}

function onFileSelect(e) {
  const file = e.target.files[0]
  if (file) readFile(file)
}

function readFile(file) {
  const reader = new FileReader()
  reader.onload = e => { form.value.compose_content = e.target.result }
  reader.readAsText(file)
}

async function parseRun() {
  if (!runCmd.value.trim()) return
  parsing.value = true
  try {
    const res = await api.parseDockerRun(runCmd.value.trim())
    parsedYaml.value = res.data.yaml
    form.value.compose_content = parsedYaml.value
    if (!form.value.name && res.data.service?.container_name) {
      form.value.name = res.data.service.container_name
    }
    toast.success('解析成功')
  } catch (e) { toast.error('解析失败: ' + e) }
  finally { parsing.value = false }
}

async function submit() {
  if (!form.value.name) { toast.error('请填写容器名称'); return }
  if (!form.value.compose_content) { toast.error('请填写 Compose 内容'); return }

  submitting.value = true
  try {
    if (props.editing) {
      await api.updateContainer(props.editing.id, form.value)
      toast.success('容器已更新')
    } else {
      await api.createContainer(form.value)
      toast.success('容器已创建并启动')
    }
    emit('created')
  } catch (e) { toast.error(typeof e === 'string' ? e : '操作失败') }
  finally { submitting.value = false }
}

onMounted(() => {
  if (props.editing) {
    form.value.name = props.editing.name
    form.value.create_mode = props.editing.create_mode || 'paste'
    form.value.compose_content = props.editing.compose_content || ''
    mode.value = props.editing.create_mode || 'paste'
  }
})
</script>

<style scoped>
.create-layout { display: flex; min-height: 520px; }
.create-sidebar {
  width: 240px;
  border-right: 1px solid var(--border);
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  flex-shrink: 0;
}
.sidebar-section { display: flex; flex-direction: column; gap: 12px; }
.form-hint { font-size: 11px; color: var(--text-muted); margin-top: 3px; }
.mode-list { display: flex; flex-direction: column; gap: 3px; }
.mode-item {
  display: flex; align-items: center; gap: 10px;
  padding: 8px 10px;
  border-radius: var(--radius);
  cursor: pointer;
  transition: all var(--transition);
  border: 1px solid transparent;
}
.mode-item:hover { background: var(--accent-dim); }
.mode-item.active { background: rgba(6,182,212,0.1); border-color: var(--border-3); color: var(--accent-light); }
.mode-name { font-size: 12.5px; font-weight: 500; }
.mode-desc { font-size: 11px; color: var(--text-muted); }
.mode-item.active .mode-desc { color: var(--accent); opacity: 0.7; }

.create-main { flex: 1; overflow: hidden; }
.mode-panel { padding: 20px; height: 100%; overflow-y: auto; display: flex; flex-direction: column; gap: 14px; }

.upload-zone {
  border: 2px dashed var(--border-2);
  border-radius: var(--radius-lg);
  padding: 40px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  transition: all var(--transition);
  text-align: center;
  min-height: 200px;
}
.upload-zone.dragging { border-color: var(--accent); background: var(--accent-dim); }
.compose-preview { display: flex; flex-direction: column; gap: 8px; }
.compose-preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 500;
}
.form-tabs { margin-bottom: 4px; }
.tab-content { padding: 14px 0; display: flex; flex-direction: column; gap: 14px; }
.list-editor { display: flex; flex-direction: column; gap: 6px; }
.list-row { display: flex; align-items: center; gap: 6px; }

.yaml-preview-bar {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 24px;
  background: var(--bg-base);
  border-top: 1px solid var(--border);
  max-height: 160px;
  overflow: hidden;
}
.yaml-preview-code {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-code);
  overflow-y: auto;
  max-height: 140px;
  flex: 1;
  white-space: pre;
}
.spin { animation: spin 0.8s linear infinite; }
</style>

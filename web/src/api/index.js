import axios from 'axios'

const api = axios.create({ baseURL: '/api' })

api.interceptors.request.use(cfg => {
  const token = localStorage.getItem('token')
  if (token) cfg.headers.Authorization = `Bearer ${token}`
  return cfg
})

api.interceptors.response.use(
  res => res.data,
  err => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      location.href = '/login'
    }
    return Promise.reject(err.response?.data?.error || err.message || 'Request failed')
  }
)

export default {
  // Auth
  systemStatus: () => api.get('/system/status'),
  setup: (data) => api.post('/auth/setup', data),
  login: (data) => api.post('/auth/login', data),

  // Dashboard
  dashboardInfo: () => api.get('/dashboard/info'),
  dashboardStats: () => api.get('/dashboard/stats'),
  dashboardRefresh: () => api.post('/dashboard/refresh'),

  // Containers
  listContainers: () => api.get('/containers'),
  createContainer: (data) => api.post('/containers', data),
  parseDockerRun: (command) => api.post('/containers/parse-run', { command }),
  getContainer: (id) => api.get(`/containers/${id}`),
  getContainerFormData: (id) => api.get(`/containers/${id}/form-data`),
  updateContainer: (id, data) => api.put(`/containers/${id}`, data),
  deleteContainer: (id) => api.delete(`/containers/${id}`),
  startContainer: (id) => api.post(`/containers/${id}/start`),
  stopContainer: (id) => api.post(`/containers/${id}/stop`),
  restartContainer: (id) => api.post(`/containers/${id}/restart`),
  containerStats: (id) => api.get(`/containers/${id}/stats`),
  containerLogs: (id, tail=500) => api.get(`/containers/${id}/logs`, { params: { tail } }),
  listContainerFiles: (id, path='/') => api.get(`/containers/${id}/files`, { params: { path } }),
  downloadContainerFile: (id, path) => `/api/containers/${id}/files/download?path=${encodeURIComponent(path)}&token=${localStorage.getItem('token')}`,
  uploadContainerFile: (id, path, file) => {
    const fd = new FormData(); fd.append('file', file)
    return api.post(`/containers/${id}/files/upload?path=${encodeURIComponent(path)}`, fd)
  },
  deleteContainerFile: (id, path) => api.delete(`/containers/${id}/files`, { params: { path } }),
  updateContainerImage: (id) => api.post(`/containers/${id}/update`),
  checkUpdates: () => api.post('/containers/check-updates'),

  // Images
  listImages: () => api.get('/images'),
  pullImage: (image) => api.post('/images/pull', { image }, { responseType: 'stream' }),
  loadImage: (file) => { const fd = new FormData(); fd.append('file', file); return api.post('/images/load', fd) },
  deleteImage: (id, force=false) => api.delete(`/images/${id}`, { params: { force } }),

  // Networks
  listNetworks: () => api.get('/networks'),
  createNetwork: (data) => api.post('/networks', data),
  deleteNetwork: (id) => api.delete(`/networks/${id}`),
  pruneNetworks: () => api.post('/networks/prune'),

  // Volumes
  listVolumes: () => api.get('/volumes'),
  createVolume: (data) => api.post('/volumes', data),
  deleteVolume: (name, force=false) => api.delete(`/volumes/${name}`, { params: { force } }),
  pruneVolumes: () => api.post('/volumes/prune'),

  // Settings
  getSettings: () => api.get('/settings'),
  updateSettings: (data) => api.put('/settings', data),
  updateAdmin: (data) => api.put('/settings/admin', data),
  installDocker: () => api.post('/settings/install-docker'),

  // WS URLs
  terminalWsUrl: (id) => `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws/containers/${id}/terminal?token=${localStorage.getItem('token')}`,
  logsWsUrl: (id) => `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws/containers/${id}/logs?token=${localStorage.getItem('token')}`,
}

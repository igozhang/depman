import http from './http'

export { authApi } from './auth'

export const readApi = {
  ui() {
    return http.get('/read/ui')
  },
  projects() {
    return http.get('/read/projects')
  },
  project(id) {
    return http.get(`/read/projects/${id}`)
  },
  environments() {
    return http.get('/read/environments')
  },
  credentials(envId, reveal = false) {
    return http.get('/read/credentials', { params: { envId: envId || undefined, reveal: reveal ? 1 : undefined } })
  },
  jobs() {
    return http.get('/read/jobs')
  },
  job(id) {
    return http.get(`/read/jobs/${id}`)
  },
  scopedKeys() {
    return http.get('/read/scoped-keys')
  }
}

export const writeApi = {
  createProject(body) {
    return http.post('/write/projects', body)
  },
  updateProject(id, body) {
    return http.put(`/write/projects/${id}`, body)
  },
  deleteProject(id) {
    return http.delete(`/write/projects/${id}`)
  },
  createEnvironment(body) {
    return http.post('/write/environments', body)
  },
  updateEnvironment(id, body) {
    return http.put(`/write/environments/${id}`, body)
  },
  deleteEnvironment(id) {
    return http.delete(`/write/environments/${id}`)
  },
  createCredential(body) {
    return http.post('/write/credentials', body)
  },
  updateCredential(id, body) {
    return http.put(`/write/credentials/${id}`, body)
  },
  deleteCredential(id) {
    return http.delete(`/write/credentials/${id}`)
  },
  upsertProjectEnv(body) {
    return http.post('/write/project-envs', body)
  },
  deleteProjectEnv(id) {
    return http.delete(`/write/project-envs/${id}`)
  },
  deploy(projectEnvId) {
    return http.post('/write/deploy', { projectEnvId })
  },
  testSSH(credId) {
    return http.post('/write/ssh/test', { credId })
  },
  createScopedKey(body) {
    return http.post('/write/scoped-keys', body)
  },
  scopedKeyPlain(id) {
    return http.get(`/write/scoped-keys/${id}/plain`)
  },
  deleteScopedKey(id) {
    return http.delete(`/write/scoped-keys/${id}`)
  }
}

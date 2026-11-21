import http from './http'

export const authApi = {
  igokey(renew = false) {
    return http.get('/auth/igokey', { params: renew ? { renew: 1 } : {} })
  },
  login(key) {
    return http.post('/auth/login', { key })
  },
  session(token) {
    return http.post('/auth/session', { token })
  }
}

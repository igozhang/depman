import axios from 'axios'

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 60000
})

let writeToken = ''
let onWriteUnauthorized = () => {}

export function setWriteToken(token) {
  writeToken = typeof token === 'string' ? token : ''
}

export function onWriteAuthLost(fn) {
  onWriteUnauthorized = typeof fn === 'function' ? fn : () => {}
}

function apiError(message, code) {
  const e = new Error(typeof message === 'string' && message ? message : 'request failed')
  if (code != null) e.code = code
  return e
}

function isWriteURL(url) {
  const u = String(url || '')
  return u.includes('/write/') || u.endsWith('/write') || u.startsWith('/write')
}

http.interceptors.request.use((config) => {
  if (writeToken) {
    config.headers = config.headers || {}
    config.headers['X-Igo-Token'] = writeToken
  }
  return config
})

http.interceptors.response.use(
  (res) => {
    const body = res.data
    if (body && typeof body.code === 'number' && body.code !== 0) {
      return Promise.reject(apiError(body.message || 'request failed', body.code))
    }
    return body?.data
  },
  (err) => {
    if (axios.isCancel?.(err) || err?.code === 'ERR_CANCELED') {
      return Promise.reject(apiError('请求已取消', 'canceled'))
    }
    const data = err.response?.data
    let msg = data?.message || err.message || 'network error'
    const code = typeof data?.code === 'number' ? data.code : err.response?.status
    if (code === 401 && isWriteURL(err.config?.url)) {
      const hdrs = err.config?.headers
      const sent =
        (typeof hdrs?.get === 'function' ? hdrs.get('X-Igo-Token') : null) ||
        hdrs?.['X-Igo-Token'] ||
        hdrs?.['x-igo-token'] ||
        ''
      if (!sent || sent === writeToken) onWriteUnauthorized(msg)
    }
    return Promise.reject(apiError(msg, code))
  }
)

export default http

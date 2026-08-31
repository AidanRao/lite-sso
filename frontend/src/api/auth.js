import axios from 'axios'
import { clearReauth, getCachedReauthToken, invalidateReauthToken, requestReauth } from '../utils/reauthCoordinator.js'

let accessToken = ''
let refreshPromise = null

export const api = axios.create({
  baseURL: '/api',
  timeout: 10000
})

const refreshClient = axios.create({
  baseURL: '/api',
  timeout: 10000,
  withCredentials: true
})

api.interceptors.request.use(config => {
  if (accessToken) {
    config.headers = config.headers || {}
    config.headers.Authorization = `Bearer ${accessToken}`
  }
  const reauthToken = getCachedReauthToken()
  if (reauthToken && config.reauth === true && !isReauthEndpoint(config.url)) {
    config.headers = config.headers || {}
    config.headers['X-Reauth-Token'] = reauthToken
  }
  config.withCredentials = true
  return config
})

api.interceptors.response.use(
  response => response.data,
  async error => {
    const originalRequest = error.config
    if (error.response?.status === 401 && originalRequest && !originalRequest._authRetry && !originalRequest.url?.includes('/auth/token/refresh')) {
      originalRequest._authRetry = true
      try {
        const refreshed = await refreshAccessToken()
        originalRequest.headers = originalRequest.headers || {}
        originalRequest.headers.Authorization = `Bearer ${refreshed}`
        return api(originalRequest)
      } catch (refreshError) {
        clearAccessToken()
      }
    }
    const machineCode = error.response?.data?.data?.code
    const descriptor = error.response?.data?.data?.reauth
    if (
      error.response?.status === 403 &&
      originalRequest &&
      !originalRequest._reauthRetry &&
      !isReauthEndpoint(originalRequest.url) &&
      descriptor &&
      (machineCode === 'REAUTH_REQUIRED' || machineCode === 'REAUTH_TOKEN_INVALID')
    ) {
      if (machineCode === 'REAUTH_TOKEN_INVALID') invalidateReauthToken()
      try {
        const token = await requestReauth(descriptor)
        originalRequest._reauthRetry = true
        originalRequest.headers = originalRequest.headers || {}
        originalRequest.headers['X-Reauth-Token'] = token
        return api(originalRequest)
      } catch (reauthError) {
        return Promise.reject(reauthError)
      }
    }
    const message = error.response?.data?.message || error.message || '请求失败'
    const apiError = new Error(message)
    apiError.status = error.response?.status
    apiError.code = error.response?.data?.code
    apiError.data = error.response?.data?.data
    return Promise.reject(apiError)
  }
)

export const setAccessToken = (token) => {
  accessToken = token || ''
}

export const clearAccessToken = () => {
  accessToken = ''
  clearReauth()
}

export const refreshAccessToken = async () => {
  if (!refreshPromise) {
    refreshPromise = refreshClient.post('/auth/token/refresh').then(response => {
      const data = response.data?.data || response.data
      setAccessToken(data?.access_token)
      return accessToken
    }).finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}

export const authAPI = {
  loginWithPassword: (data) => {
    return api.post('/auth/login/password', data)
  },

  loginWithEmail: (data) => {
    return api.post('/auth/login/email', data)
  },

  sendEmailCode: (data) => {
    return api.post('/auth/email/send', data)
  },

  refreshToken: () => refreshAccessToken(),

  register: (data) => {
    return api.post('/user/register', data)
  },

  resetPassword: (data) => {
    return api.post('/user/password/reset', data)
  },

  getCaptcha: () => {
    return api.get('/auth/captcha')
  },

  getQRCode: (redirect) => {
    return api.get('/auth/qr/generate', { params: { redirect } })
  },

  checkQRCode: (code) => {
    return api.get('/auth/qr/poll', { params: { code } })
  },

  completeQRCode: (data) => {
    return api.post('/auth/qr/complete', data)
  }
}

export const userAPI = {
  getAuditLogs: (params, signal) => api.get('/user/audit-logs', { params, signal }),

  getProfile: () => {
    return api.get('/user/profile')
  },

  getLoginMethods: () => {
    return api.get('/user/login-methods')
  },

  changePassword: (data) => {
    return api.put('/user/password', data)
  },

  getApplications: () => {
    return api.get('/user/applications')
  },

  updateProfile: (data) => {
    return api.put('/user/profile', data)
  },

  uploadAvatar: (file) => {
    const data = new FormData()
    data.append('file', file)
    return api.post('/user/avatar', data, { timeout: 15000 })
  },

  getDevices: () => {
    return api.get('/user/devices')
  },

  getEmails: () => api.get('/user/emails'),

  addEmail: (email) => api.post('/user/emails', { email }, { reauth: true }),

  resendEmailVerification: (id) => api.post(`/user/emails/${encodeURIComponent(id)}/verification`),

  confirmEmailVerification: (token) => api.post('/user/emails/verification/confirm', { token }),

  setPrimaryEmail: (id) => api.put(`/user/emails/${encodeURIComponent(id)}/primary`, null, { reauth: true }),

  deleteEmail: (id) => api.delete(`/user/emails/${encodeURIComponent(id)}`, { reauth: true }),

  revokeDevice: (deviceID) => {
    return api.delete(`/user/devices/${encodeURIComponent(deviceID)}`)
  },

  unbindThirdParty: (provider) => {
    return api.delete(`/user/third/${provider}`, { reauth: true })
  },

  getThirdPartyBindingPreview: (bindingID) => {
    return api.get(`/user/third/bindings/${encodeURIComponent(bindingID)}`)
  },

  confirmThirdPartyBinding: (bindingID) => {
    return api.post(`/user/third/bindings/${encodeURIComponent(bindingID)}/confirm`, null, { reauth: true })
  }
}

export const passkeyAPI = {
  list: () => api.get('/user/passkeys'),
  sendRegistrationEmail: (data) => api.post('/user/passkeys/registration/email/send', data),
  registrationOptions: (data) => api.post('/user/passkeys/registration/options', data),
  registrationVerify: (data) => api.post('/user/passkeys/registration/verify', data),
  rename: (id, name) => api.patch(`/user/passkeys/${encodeURIComponent(id)}`, { name }),
  remove: (id) => api.delete(`/user/passkeys/${encodeURIComponent(id)}`, { reauth: true })
}

export const reauthAPI = {
  passkeyOptions: () => api.post('/user/reauth/passkey/options'),
  passkeyVerify: (data) => api.post('/user/reauth/passkey/verify', data),
  sendEmail: (data) => api.post('/user/reauth/email/send', data),
  verifyEmail: (data) => api.post('/user/reauth/email/verify', data)
}

const isReauthEndpoint = (url = '') => String(url).includes('/user/reauth/')

export const adminAPI = {
  listUsers: () => {
    return api.get('/admin/users')
  },

  getUserDetail: (id) => {
    return api.get(`/admin/users/${id}`)
  },

  listOAuthClients: () => {
    return api.get('/admin/oauth-clients')
  },

  getOAuthClientSecret: (id) => {
    return api.get(`/admin/oauth-clients/${id}/secret`)
  },

  createOAuthClient: (data) => {
    return api.post('/admin/oauth-clients', data)
  },

  updateOAuthClient: (id, data) => {
    return api.put(`/admin/oauth-clients/${id}`, data)
  },

  uploadOAuthClientLogo: (id, file) => {
    const data = new FormData()
    data.append('file', file)
    return api.post(`/admin/oauth-clients/${id}/logo`, data, { timeout: 15000 })
  },

  clearOAuthClientLogo: (id) => {
    return api.delete(`/admin/oauth-clients/${id}/logo`)
  }
}

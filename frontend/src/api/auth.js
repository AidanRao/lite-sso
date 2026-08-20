import axios from 'axios'

let accessToken = ''
let refreshPromise = null

const api = axios.create({
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
  getProfile: () => {
    return api.get('/user/profile')
  },

  updateProfile: (data) => {
    return api.put('/user/profile', data)
  },

  unbindThirdParty: (provider) => {
    return api.delete(`/user/third/${provider}`)
  }
}

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
  }
}

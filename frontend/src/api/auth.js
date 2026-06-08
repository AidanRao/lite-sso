import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000
})

api.interceptors.response.use(
  response => response.data,
  error => {
    const message = error.response?.data?.message || error.message || '请求失败'
    const apiError = new Error(message)
    apiError.status = error.response?.status
    apiError.code = error.response?.data?.code
    apiError.data = error.response?.data?.data
    return Promise.reject(apiError)
  }
)

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

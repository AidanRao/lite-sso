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
    return api.post('/auth/register', data)
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

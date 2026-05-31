<template>
  <div class="min-h-screen md:h-screen flex overflow-hidden">
    <router-link to="/docs" class="fixed left-4 top-4 z-20 text-sm font-medium text-gray-400 hover:text-[#0891b2] transition-colors md:hidden">
      开发文档
    </router-link>

    <AuthSplashPane class="hidden md:flex flex-[1.5]" title="身份认证系统" :target-name="targetPlatformName" />

    <div class="login-auth-pane flex-1 min-h-screen md:h-screen bg-gradient-to-br from-[#ecfeff] to-[#f0fdfa] flex items-center justify-center overflow-y-auto p-4 sm:p-6 lg:p-8">
      <div class="login-stack w-full max-w-md py-6 md:py-0">
        <div class="login-card bg-white rounded-2xl shadow-xl shadow-[#0891b2]/5 p-6 sm:p-8 xl:p-10">
          <div class="login-header text-center mb-6 xl:mb-8">
            <h2 class="text-2xl font-bold text-gray-800">欢迎回来</h2>
            <p v-if="targetClientName" class="text-gray-500 text-sm mt-2">
              登录后将进入 <span class="font-semibold text-[#0891b2]">{{ targetClientName }}</span>
            </p>
            <p v-else class="text-gray-500 text-sm mt-2">请登录您的账号</p>
          </div>

          <div v-if="errorMessage" class="mb-4 p-4 bg-red-50 border border-red-200 rounded-xl text-red-600 text-sm flex items-center gap-2">
            <AlertTriangle class="w-4 h-4 flex-shrink-0" />
            {{ errorMessage }}
          </div>

          <div class="login-tabs flex mb-6 xl:mb-8 border-b border-gray-100">
            <button
              v-for="tab in tabs"
              :key="tab.key"
              @click="activeTab = tab.key"
              class="flex-1 py-3 text-sm font-medium transition-all relative"
              :class="activeTab === tab.key ? 'text-[#0891b2]' : 'text-gray-500 hover:text-gray-700'"
            >
              {{ tab.label }}
              <span v-if="activeTab === tab.key" class="absolute bottom-0 left-1/2 -translate-x-1/2 w-8 h-0.5 bg-[#0891b2] rounded-full"></span>
            </button>
          </div>

          <div class="login-form-panel min-h-[260px] xl:min-h-[292px] overflow-hidden">
            <div v-show="activeTab === 'password'" class="space-y-5">
              <el-form :model="passwordForm" :rules="passwordRules" ref="pwdFormRef" @submit.prevent="handlePasswordLogin">
                <el-form-item prop="email" class="mb-5">
                  <label class="block text-sm font-medium text-gray-700 mb-2">邮箱</label>
                  <el-input
                    v-model="passwordForm.email"
                    type="email"
                    placeholder="请输入邮箱"
                    :prefix-icon="Mail"
                    class="h-12"
                  />
                </el-form-item>
                <div class="mb-3">
                  <div class="flex w-full items-center justify-between mb-2">
                    <label class="block text-sm font-medium text-gray-700">密码</label>
                    <router-link to="/reset-password" class="text-sm font-medium text-[#0891b2] hover:text-[#0e7490] transition-colors">
                      忘记密码？
                    </router-link>
                  </div>
                  <el-form-item prop="password" class="mb-0">
                    <el-input
                      v-model="passwordForm.password"
                      :type="showPassword ? 'text' : 'password'"
                      placeholder="请输入密码"
                      :prefix-icon="Lock"
                      class="h-12 w-full password-input"
                    >
                      <template #suffix>
                        <button
                          type="button"
                          class="w-8 h-8 inline-flex items-center justify-center rounded-lg text-gray-400 hover:text-[#0891b2] hover:bg-cyan-50 transition-colors"
                          :aria-label="showPassword ? '隐藏密码' : '显示密码'"
                          @click.stop="showPassword = !showPassword"
                        >
                          <EyeOff v-if="showPassword" class="w-4 h-4" />
                          <Eye v-else class="w-4 h-4" />
                        </button>
                      </template>
                    </el-input>
                  </el-form-item>
                </div>
                <div class="pt-4">
                  <button
                    type="submit"
                    class="w-full h-12 text-white rounded-xl font-semibold text-base transition-colors flex items-center justify-center disabled:cursor-not-allowed"
                    :class="[passwordButtonClass, passwordLoginInvalid ? 'cursor-not-allowed' : '']"
                    :disabled="passwordLoginDisabled"
                  >
                    <svg v-if="loading" class="animate-spin h-5 w-5 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    {{ passwordLoginButtonText }}
                  </button>
                </div>
              </el-form>
            </div>

            <div v-show="activeTab === 'email'" class="space-y-5">
              <el-form :model="emailForm" :rules="emailRules" ref="emailFormRef" @submit.prevent="handleEmailLogin">
              <el-form-item prop="email" class="mb-5">
                <label class="block text-sm font-medium text-gray-700 mb-2">邮箱</label>
                <el-input
                  v-model="emailForm.email"
                  type="email"
                  placeholder="请输入邮箱"
                  :prefix-icon="Mail"
                  class="h-12"
                />
              </el-form-item>
              <el-form-item prop="code" class="mb-6">
                <label class="block text-sm font-medium text-gray-700 mb-2">邮箱验证码</label>
                <div class="flex gap-3">
                  <el-input
                    v-model="emailForm.code"
                    type="text"
                    placeholder="请输入邮箱验证码"
                    maxlength="6"
                    :prefix-icon="MessageSquare"
                    class="h-12 flex-1"
                  />
                  <el-button
                    type="default"
                    @click="openSendCodeModal"
                    :disabled="countdown > 0"
                    class="h-12 px-6"
                  >
                    {{ countdown > 0 ? `${countdown}秒后重发` : '发送验证码' }}
                  </el-button>
                </div>
              </el-form-item>
              <div class="pt-1">
                <button type="submit" class="w-full h-12 bg-[#0891b2] text-white rounded-xl font-semibold text-base hover:bg-[#0e7490] transition-colors flex items-center justify-center" :disabled="loading">
                  <svg v-if="loading" class="animate-spin h-5 w-5 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  {{ loading ? '登录中...' : '登录' }}
                </button>
              </div>
            </el-form>
          </div>

          <div v-show="activeTab === 'qr'" class="flex flex-col items-center justify-center py-4">
            <div class="w-44 h-44 mx-auto bg-gray-50 rounded-xl flex items-center justify-center mb-6 shadow-inner">
              <div v-if="qrCodeUrl" class="p-4 text-center">
                <div class="text-xs text-gray-400 mb-2">登录码</div>
                <div class="text-sm font-mono text-gray-700 break-all">{{ qrCodeUrl }}</div>
              </div>
              <div v-else class="flex flex-col items-center text-gray-400">
                <svg class="w-12 h-12 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                </svg>
                <span class="text-sm">加载中...</span>
              </div>
            </div>
            <p class="text-gray-600 text-sm mb-2">{{ qrStatus }}</p>
            <p v-if="qrTimer > 0" class="text-gray-400 text-xs">{{ Math.floor(qrTimer / 60) }}:{{ String(qrTimer % 60).padStart(2, '0') }}</p>
            <el-button type="default" @click="refreshQRCode" class="mt-4">
              刷新二维码
            </el-button>
          </div>
        </div>

        <div class="login-divider my-4 xl:my-6 flex items-center gap-4">
            <div class="flex-1 h-px bg-gradient-to-r from-transparent via-gray-200 to-transparent"></div>
            <span class="text-gray-400 text-sm font-medium">其他登录方式</span>
            <div class="flex-1 h-px bg-gradient-to-r from-transparent via-gray-200 to-transparent"></div>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="provider in oauthProviders"
              :key="provider.id"
              @click="oauthLogin(provider.id)"
              class="login-provider-button w-full flex items-center justify-center gap-2 border-2 border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 hover:shadow-sm transition-all duration-200 text-gray-700 font-medium"
            >
              <span v-html="provider.icon" class="login-provider-icon flex-shrink-0"></span>
              <span>{{ provider.name }} 登录</span>
            </button>
          </div>

          <p class="login-register-link text-center text-gray-500 text-sm mt-5 xl:mt-8">
            还没有账号？<router-link :to="registerRoute" class="text-[#0891b2] hover:text-[#0e7490] font-medium transition-colors">立即注册</router-link>
          </p>
        </div>
      </div>
    </div>
  </div>

  <SendCodeModal
    :visible="showSendCodeModal"
    :email="emailForm.email"
    @close="showSendCodeModal = false"
    @success="handleSendCodeSuccess"
  />
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Mail, Lock, Eye, EyeOff, MessageSquare, AlertTriangle } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { authAPI } from '../api/auth'
import AuthSplashPane from '../components/AuthSplashPane.vue'
import SendCodeModal from '../components/SendCodeModal.vue'
import { getLoginRedirect, loadTargetClientName } from '../utils/oauthTarget'

const route = useRoute()

const redirectUrl = ref(getLoginRedirect(route))
const targetClientName = ref('')

const tabs = [
  { key: 'password', label: '密码登录' },
  { key: 'email', label: '邮箱登录' },
  { key: 'qr', label: '扫码登录' },
]

const oauthProviders = [
  {
    id: 'github',
    name: 'GitHub',
    icon: `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>`
  },
  {
    id: 'feishu',
    name: '飞书',
    icon: `<svg width="20" height="20" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M10 8c0 1 7 3.5 14.745 16.744 0 0 4.184-4.363 6.255-5.744 1.5-1 2.712-1.332 2.712-1.332C33.712 15.156 29.5 8 28 8z" fill="#00d6b9"/><path d="M43.5 18.5c-1-.667-3.65-1.771-6.5-1.5a15 15 0 0 0-3.288.668S32.5 18 31 19c-2.07 1.38-6.255 5.744-6.255 5.744-1.428 1.397-3.05 2.732-5.245 3.756 0 0 7 3 11.5 3 5.063 0 7-3.5 7-3.5 1.5-3.305 3.5-7 5.5-9.5" fill="#163c9a"/><path d="M4 17.5v17c0 1 6 5.5 15 5.5 10 0 17.05-7.705 19-12 0 0-1.937 3.5-7 3.5-4.5 0-11.5-3-11.5-3-5.117-2.239-10.03-6.577-12.906-9.117C4.974 17.953 4 17.093 4 17.5" fill="#3370ff"/></svg>`
  }
]

const activeTab = ref('password')
const showPassword = ref(false)
const loading = ref(false)
const errorMessage = ref('')
const passwordLockSeconds = ref(0)
const countdown = ref(0)
const showSendCodeModal = ref(false)
const qrCodeUrl = ref('')
const qrStatus = ref('请使用 Lite SSO App 扫描二维码')
const qrTimer = ref(0)
const qrCheckInterval = ref(null)
const qrTimerInterval = ref(null)
const passwordLockInterval = ref(null)
const currentQRID = ref('')

const pwdFormRef = ref(null)
const emailFormRef = ref(null)

const passwordForm = ref({
  email: '',
  password: ''
})

const emailForm = ref({
  email: '',
  code: ''
})

const passwordEmailValid = computed(() => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(passwordForm.value.email.trim()))
const passwordValueValid = computed(() => passwordForm.value.password.length >= 3)
const passwordFormValid = computed(() => passwordEmailValid.value && passwordValueValid.value)
const passwordLoginInvalid = computed(() => !passwordFormValid.value && passwordLockSeconds.value <= 0 && !loading.value)
const passwordLoginDisabled = computed(() => loading.value || passwordLockSeconds.value > 0)
const targetPlatformName = computed(() => targetClientName.value || 'Lite SSO')
const registerRoute = computed(() => {
  if (!redirectUrl.value || redirectUrl.value === '/profile') {
    return '/register'
  }

  return {
    path: '/register',
    query: {
      redirect: redirectUrl.value
    }
  }
})
const passwordButtonClass = computed(() => (
  passwordLockSeconds.value > 0
    ? 'bg-gray-300 hover:bg-gray-300'
    : 'bg-[#0891b2] hover:bg-[#0e7490]'
))
const passwordLoginButtonText = computed(() => {
  if (loading.value) {
    return '登录中...'
  }
  if (passwordLockSeconds.value > 0) {
    return `${passwordLockSeconds.value} 秒后重试`
  }
  return '登录'
})

watch(() => passwordForm.value.email, (email) => {
  if (emailForm.value.email !== email) {
    emailForm.value.email = email
  }
  stopPasswordLockCountdown()
  errorMessage.value = ''
})

watch(() => emailForm.value.email, (email) => {
  if (passwordForm.value.email !== email) {
    passwordForm.value.email = email
  }
  errorMessage.value = ''
})

const passwordRules = {
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱地址', trigger: ['blur', 'change'] }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 3, message: '密码长度不能小于3位', trigger: 'blur' }
  ]
}

const emailRules = {
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱地址', trigger: ['blur', 'change'] }
  ],
  code: [
    { required: true, message: '请输入验证码', trigger: 'blur' },
    { len: 6, message: '验证码为6位', trigger: 'blur' }
  ]
}

const responseData = (response) => response?.data || response || {}

const loadTargetClient = async () => {
  targetClientName.value = await loadTargetClientName(redirectUrl.value)
}

const finishLogin = (response) => {
  const data = responseData(response)
  window.location.href = data.redirect_url || redirectUrl.value || '/profile'
}

const stopPasswordLockCountdown = () => {
  if (passwordLockInterval.value) {
    clearInterval(passwordLockInterval.value)
    passwordLockInterval.value = null
  }
  passwordLockSeconds.value = 0
}

const startPasswordLockCountdown = (seconds) => {
  stopPasswordLockCountdown()
  passwordLockSeconds.value = Math.max(1, Number(seconds) || 1)
  passwordLockInterval.value = setInterval(() => {
    passwordLockSeconds.value -= 1
    if (passwordLockSeconds.value <= 0) {
      stopPasswordLockCountdown()
      return
    }
  }, 1000)
}

const handlePasswordLogin = async () => {
  try {
    if (passwordLoginDisabled.value || !pwdFormRef.value) return
    try {
      await pwdFormRef.value.validate()
    } catch (error) {
      return
    }
    loading.value = true
    errorMessage.value = ''
    const response = await authAPI.loginWithPassword({
      ...passwordForm.value,
      redirect: redirectUrl.value
    })
    ElMessage.success('登录成功，正在跳转...')
    setTimeout(() => {
      finishLogin(response)
    }, 1000)
  } catch (error) {
    if (error.status === 429) {
      const retryAfterSeconds = Math.max(1, Number(error.data?.retry_after_seconds) || 1)
      ElMessage.error(`密码错误次数过多，请 ${retryAfterSeconds} 秒后再试`)
      startPasswordLockCountdown(retryAfterSeconds)
      passwordForm.value.password = ''
      return
    }
    ElMessage.error(error.message || '邮箱或密码错误')
  } finally {
    loading.value = false
  }
}

const openSendCodeModal = () => {
  if (!emailForm.value.email) {
    ElMessage.warning('请先输入邮箱')
    return
  }
  showSendCodeModal.value = true
}

const handleSendCodeSuccess = () => {
  ElMessage.success('验证码已发送')
  countdown.value = 60
  const timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      clearInterval(timer)
    }
  }, 1000)
}

const handleEmailLogin = async () => {
  try {
    if (!emailFormRef.value) return
    await emailFormRef.value.validate()
    loading.value = true
    errorMessage.value = ''
    const response = await authAPI.loginWithEmail({
      email: emailForm.value.email,
      otp: emailForm.value.code,
      redirect: redirectUrl.value
    })
    ElMessage.success('登录成功，正在跳转...')
    setTimeout(() => {
      finishLogin(response)
    }, 1000)
  } catch (error) {
    errorMessage.value = error.message || '登录失败'
  } finally {
    loading.value = false
  }
}

const oauthLogin = (provider) => {
  const redirectParam = `?redirect=${encodeURIComponent(redirectUrl.value)}`
  window.location.href = `/api/auth/third/${provider}${redirectParam}`
}

const refreshQRCode = async () => {
  try {
    const response = await authAPI.getQRCode(redirectUrl.value)
    const data = responseData(response)
    qrCodeUrl.value = data.code
    currentQRID.value = data.code
    qrStatus.value = '请使用 Lite SSO App 扫描二维码'
    qrTimer.value = 300

    if (qrCheckInterval.value) clearInterval(qrCheckInterval.value)
    if (qrTimerInterval.value) clearInterval(qrTimerInterval.value)

    qrCheckInterval.value = setInterval(async () => {
      try {
        const checkResponse = await authAPI.checkQRCode(currentQRID.value)
        const checkData = responseData(checkResponse)
        if (checkData.status === 'scanned') {
          qrStatus.value = '已扫描，请在 App 中确认'
        }
        if (checkData.status === 'confirmed' && checkData.login_ticket) {
          clearInterval(qrCheckInterval.value)
          clearInterval(qrTimerInterval.value)
          const completeResponse = await authAPI.completeQRCode({
            code: currentQRID.value,
            login_ticket: checkData.login_ticket
          })
          qrStatus.value = '登录成功'
          ElMessage.success('登录成功，正在跳转...')
          setTimeout(() => {
            finishLogin(completeResponse)
          }, 1000)
        }
      } catch (error) {
        console.error('检查二维码状态失败:', error)
      }
    }, 2000)

    qrTimerInterval.value = setInterval(() => {
      qrTimer.value--
      if (qrTimer.value <= 0) {
        clearInterval(qrCheckInterval.value)
        clearInterval(qrTimerInterval.value)
        qrStatus.value = '二维码已过期，请刷新'
      }
    }, 1000)
  } catch (error) {
    ElMessage.error(error.message || '获取二维码失败')
  }
}

onMounted(() => {
  loadTargetClient()
  if (activeTab.value === 'qr') {
    refreshQRCode()
  }
})

onUnmounted(() => {
  if (qrCheckInterval.value) clearInterval(qrCheckInterval.value)
  if (qrTimerInterval.value) clearInterval(qrTimerInterval.value)
  if (passwordLockInterval.value) clearInterval(passwordLockInterval.value)
})

watch(activeTab, (tab) => {
  stopPasswordLockCountdown()
  errorMessage.value = ''
  if (tab === 'qr' && !currentQRID.value) {
    refreshQRCode()
  }
})
</script>

<style scoped>
@media (max-height: 760px) and (min-width: 768px) {
  .login-auth-pane {
    align-items: flex-start;
    padding-bottom: 16px;
    padding-top: 16px;
  }

  .login-stack {
    padding-bottom: 0;
    padding-top: 0;
  }

  .login-card {
    padding: 24px;
  }

  .login-header,
  .login-tabs {
    margin-bottom: 16px;
  }

  .login-form-panel {
    min-height: 238px;
  }

  .login-divider {
    margin-bottom: 10px;
    margin-top: 10px;
  }

  .login-provider-button {
    padding-bottom: 7px;
    padding-top: 7px;
  }

  .login-register-link {
    margin-top: 14px;
  }
}

.login-provider-button {
  height: 48px;
  border-radius: 8px;
  padding: 0 14px;
}

.login-provider-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
}

.login-provider-icon :deep(svg) {
  width: 20px;
  height: 20px;
}

@media (max-height: 680px) and (min-width: 768px) {
  .login-card {
    padding: 20px;
  }

  .login-header,
  .login-tabs {
    margin-bottom: 12px;
  }

  .login-form-panel {
    min-height: 226px;
  }
}
</style>

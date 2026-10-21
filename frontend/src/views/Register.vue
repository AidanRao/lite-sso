<template>
  <div class="register-page min-h-screen md:h-screen flex overflow-hidden">
    <AuthSplashPane class="hidden md:flex flex-[1.5]" title="身份认证系统" :target-name="targetPlatformName" :show-docs-link="false" />

    <div class="register-auth-pane flex-1 min-w-0 bg-gradient-to-br from-[#ecfeff] to-[#f0fdfa] flex flex-col overflow-y-auto p-4 sm:p-6 lg:p-8">
      <div class="register-stack w-full max-w-md mx-auto my-auto shrink-0 py-6 md:py-0">
        <div class="register-card bg-white rounded-2xl shadow-xl shadow-[#0891b2]/5 p-6 sm:p-8 xl:p-10">
          <div class="register-header text-center mb-6 xl:mb-8">
            <h1 class="register-title">注册账号</h1>
          </div>

          <div v-if="errorMessage" role="alert" class="register-notice mb-4 p-4 bg-red-50 border border-red-200 rounded-xl text-red-600 text-sm flex items-center gap-2">
            <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {{ errorMessage }}
          </div>
          <div v-if="successMessage" role="status" class="register-notice mb-4 p-4 bg-green-50 border border-green-200 rounded-xl text-green-600 text-sm flex items-center gap-2">
            <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
            {{ successMessage }}
          </div>

          <el-form :model="form" @submit.prevent="handleRegister" class="register-form">
            <el-form-item class="mb-5">
              <label for="register-email" class="block text-sm font-medium text-gray-700 mb-2">邮箱</label>
              <el-input
                id="register-email"
                v-model="form.email"
                type="email"
                placeholder="请输入邮箱"
                :prefix-icon="Mail"
                class="h-12"
              />
            </el-form-item>

            <el-form-item class="mb-5">
              <label for="register-password" class="block text-sm font-medium text-gray-700 mb-2">密码</label>
              <el-input
                id="register-password"
                v-model="form.password"
                :type="showPassword ? 'text' : 'password'"
                placeholder="请输入密码（至少10位，含英文和数字）"
                :prefix-icon="Lock"
                class="h-12 password-input"
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

            <el-form-item class="mb-5">
              <label for="register-confirm-password" class="block text-sm font-medium text-gray-700 mb-2">确认密码</label>
              <el-input
                id="register-confirm-password"
                v-model="form.confirmPassword"
                :type="showConfirmPassword ? 'text' : 'password'"
                placeholder="请再次输入密码"
                :prefix-icon="Lock"
                class="h-12 password-input"
              >
                <template #suffix>
                  <button
                    type="button"
                    class="w-8 h-8 inline-flex items-center justify-center rounded-lg text-gray-400 hover:text-[#0891b2] hover:bg-cyan-50 transition-colors"
                    :aria-label="showConfirmPassword ? '隐藏确认密码' : '显示确认密码'"
                    @click.stop="showConfirmPassword = !showConfirmPassword"
                  >
                    <EyeOff v-if="showConfirmPassword" class="w-4 h-4" />
                    <Eye v-else class="w-4 h-4" />
                  </button>
                </template>
              </el-input>
            </el-form-item>

            <el-form-item class="mb-5">
              <label for="register-code" class="block text-sm font-medium text-gray-700 mb-2">邮箱验证码</label>
              <div class="register-code-row flex w-full min-w-0 gap-3">
                <el-input
                  id="register-code"
                  v-model="form.code"
                  type="text"
                  placeholder="请输入邮箱验证码"
                  maxlength="6"
                  :prefix-icon="MessageSquare"
                  class="h-12 min-w-0 flex-1"
                />
                <el-button
                  type="default"
                  @click="openSendCodeModal"
                  :disabled="countdown > 0"
                  class="register-send-code h-12 shrink-0 px-4"
                >
                  {{ countdown > 0 ? `${countdown}秒后重发` : '发送验证码' }}
                </el-button>
              </div>
            </el-form-item>

            <button type="submit" class="register-submit w-full h-12 bg-[#0891b2] text-white rounded-xl font-semibold text-base hover:bg-[#0e7490] transition-colors flex items-center justify-center disabled:cursor-not-allowed" :disabled="loading">
              <svg v-if="loading" class="animate-spin h-5 w-5 mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              {{ loading ? '注册中...' : '注册' }}
            </button>
          </el-form>

          <div class="register-divider mt-6 mb-4 xl:mb-6 flex items-center gap-4">
            <div class="flex-1 h-px bg-gradient-to-r from-transparent via-gray-200 to-transparent"></div>
            <span class="text-gray-400 text-sm font-medium">其他注册方式</span>
            <div class="flex-1 h-px bg-gradient-to-r from-transparent via-gray-200 to-transparent"></div>
          </div>

          <div class="register-provider-list flex items-center justify-around">
            <button
              v-for="provider in oauthProviders"
              :key="provider.id"
              type="button"
              @click="oauthRegister(provider.id)"
              :aria-label="`${provider.name}注册`"
              :title="`${provider.name}注册`"
              class="register-provider-button flex items-center justify-center border-2 border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 hover:shadow-sm transition-all duration-200"
            >
              <ThirdPartyProviderIcon :provider="provider.id" :size="24" class="register-provider-icon shrink-0" />
            </button>
          </div>

          <p class="register-login-link text-center text-gray-500 text-sm mt-5 xl:mt-8">
            已有账号？<router-link :to="loginRoute" class="text-[#0891b2] hover:text-[#0e7490] font-medium transition-colors">立即登录</router-link>
          </p>
        </div>
      </div>
    </div>
  </div>

  <SendCodeModal
    :visible="showSendCodeModal"
    :email="form.email"
    purpose="REGISTER"
    @close="showSendCodeModal = false"
    @success="handleSendCodeSuccess"
  />
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Mail, Lock, Eye, EyeOff, MessageSquare } from 'lucide-vue-next'
import { authAPI, setAccessToken } from '../api/auth'
import AuthSplashPane from '../components/AuthSplashPane.vue'
import SendCodeModal from '../components/SendCodeModal.vue'
import ThirdPartyProviderIcon from '../components/ThirdPartyProviderIcon.vue'
import { getLoginRedirect, loadTargetClientName } from '../utils/oauthTarget'
import { passwordPolicyError } from '../utils/passwordPolicy'

const route = useRoute()
const redirectUrl = ref(getLoginRedirect(route))
const targetClientName = ref('')
const loading = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const showPassword = ref(false)
const showConfirmPassword = ref(false)
const countdown = ref(0)
const showSendCodeModal = ref(false)
const challengeId = ref('')

const oauthProviders = [
  { id: 'github', name: 'GitHub' },
  { id: 'feishu', name: '飞书' },
  { id: 'qq', name: 'QQ' },
  { id: 'google', name: 'Google' }
]

const form = ref({
  email: '',
  password: '',
  confirmPassword: '',
  code: ''
})

const targetPlatformName = computed(() => targetClientName.value || 'SSO')
const loginRoute = computed(() => {
  if (!redirectUrl.value || redirectUrl.value === '/profile') {
    return '/login'
  }

  return {
    path: '/login',
    query: {
      redirect: redirectUrl.value
    }
  }
})

const loadTargetClient = async () => {
  targetClientName.value = await loadTargetClientName(redirectUrl.value)
}

const oauthRegister = (provider) => {
  const redirectParam = `?redirect=${encodeURIComponent(redirectUrl.value)}`
  window.location.href = `/api/auth/third/${provider}${redirectParam}`
}

const openSendCodeModal = () => {
  if (!form.value.email) {
    errorMessage.value = '请先输入邮箱'
    return
  }
  showSendCodeModal.value = true
}

const handleSendCodeSuccess = (data) => {
  challengeId.value = data.challenge_id || ''
  successMessage.value = '验证码已发送'
  errorMessage.value = ''
  countdown.value = 60
  const timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      clearInterval(timer)
    }
  }, 1000)
}

const handleRegister = async () => {
  try {
    if (form.value.password !== form.value.confirmPassword) {
      errorMessage.value = '两次输入的密码不一致'
      return
    }

    const policyError = passwordPolicyError(form.value.password)
    if (policyError) {
      errorMessage.value = policyError
      return
    }

    loading.value = true
    errorMessage.value = ''
    successMessage.value = ''

    const response = await authAPI.register({
      email: form.value.email,
      password: form.value.password,
      challenge_id: challengeId.value,
      code: form.value.code
    })

    const data = response.data || response
    setAccessToken(data.access_token)

    successMessage.value = '注册成功，正在登录...'
    setTimeout(() => {
      window.location.href = redirectUrl.value || '/profile'
    }, 1000)
  } catch (error) {
    errorMessage.value = error.message
    successMessage.value = ''
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadTargetClient()
})
</script>

<style scoped>
.register-title {
  color: #1f2937;
  font-size: 24px;
  font-weight: 600;
  line-height: 1.35;
}

.register-form :deep(.el-form-item__content) {
  display: block;
}

.register-form label {
  line-height: 20px;
}

.register-form :deep(.el-input) {
  min-width: 0;
}

.register-provider-button {
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  border-radius: 50%;
  padding: 0;
}

.register-provider-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
}

.register-provider-icon :deep(svg) {
  width: 24px;
  height: 24px;
}

@media (max-width: 767px) {
  .register-page {
    min-height: 100dvh;
  }

  .register-auth-pane {
    background: #fff;
    padding: env(safe-area-inset-top) env(safe-area-inset-right) calc(env(safe-area-inset-bottom) + 16px) env(safe-area-inset-left);
  }

  .register-stack {
    max-width: none;
    padding-block: 0;
  }

  .register-card {
    padding: 16px 24px 24px;
    border-radius: 0;
    box-shadow: none;
  }

  .register-header {
    margin-bottom: 24px;
  }

  .register-form :deep(.el-form-item) {
    margin-bottom: 16px;
  }

  .register-form label {
    margin-bottom: 6px;
  }

  .register-form :deep(.el-input__inner) {
    font-size: 16px;
  }

  .register-divider {
    margin-top: 28px;
    margin-bottom: 16px;
  }

  .register-login-link {
    margin-top: 16px;
  }

  .register-notice {
    padding: 8px 12px;
    overflow-wrap: anywhere;
  }
}

@media (max-width: 767px) and (max-height: 760px) {
  .register-card {
    padding: 8px 24px 16px;
  }

  .register-header {
    margin-bottom: 12px;
  }

  .register-title {
    font-size: 22px;
  }

  .register-form :deep(.el-form-item) {
    margin-bottom: 12px;
  }

  .register-form label {
    margin-bottom: 4px;
  }

  .register-form :deep(.h-12) {
    height: 44px;
  }

  .register-divider {
    margin-top: 20px;
    margin-bottom: 12px;
  }

  .register-login-link {
    margin-top: 12px;
  }
}
</style>

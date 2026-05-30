<template>
  <div class="min-h-screen flex overflow-hidden">
    <div class="flex-[1.5] bg-gradient-to-br from-[#0891b2] via-[#0e7490] to-[#164e63] flex flex-col items-center justify-center p-12 relative overflow-hidden">
      <div class="absolute inset-0 bg-radial-gradient-circle opacity-15"></div>
      <div class="absolute top-10 left-10 w-32 h-32 bg-white/5 rounded-full blur-3xl"></div>
      <div class="absolute bottom-10 right-10 w-48 h-48 bg-cyan-300/10 rounded-full blur-3xl"></div>
      
      <div class="relative z-10 text-center text-white">
        <div class="mb-8">
          <div class="inline-flex items-center justify-center w-16 h-16 bg-white/20 backdrop-blur-sm rounded-2xl mb-6">
            <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
            </svg>
          </div>
        </div>
        <h1 class="text-5xl font-bold mb-4 tracking-tight">Lite SSO</h1>
        <p class="text-lg opacity-90 max-w-md leading-relaxed">
          正在前往
          <span class="font-semibold">{{ targetPlatformName }}</span>
        </p>
        <div class="mt-12 flex justify-center">
          <img 
            src="/assets/images/default.png" 
            alt="SSO Illustration" 
            class="w-72 h-72 rounded-2xl shadow-2xl object-cover hover:scale-105 transition-all duration-500"
          />
        </div>
      </div>
    </div>

    <div class="flex-1 bg-gradient-to-br from-[#ecfeff] to-[#f0fdfa] flex items-center justify-center p-8">
      <div class="w-full max-w-md">
        <div class="bg-white rounded-2xl shadow-xl shadow-[#0891b2]/5 p-8 sm:p-10">
          <div class="text-center mb-8">
            <h2 class="text-2xl font-bold text-gray-800">创建账号</h2>
            <p class="text-gray-500 text-sm mt-2">开启您的 SSO 之旅</p>
          </div>

          <div v-if="errorMessage" class="mb-4 p-4 bg-red-50 border border-red-200 rounded-xl text-red-600 text-sm flex items-center gap-2">
            <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {{ errorMessage }}
          </div>
          <div v-if="successMessage" class="mb-4 p-4 bg-green-50 border border-green-200 rounded-xl text-green-600 text-sm flex items-center gap-2">
            <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
            {{ successMessage }}
          </div>

          <el-form :model="form" @submit.prevent="handleRegister" class="space-y-5">
            <el-form-item class="mb-5">
              <label class="block text-sm font-medium text-gray-700 mb-2">邮箱</label>
              <el-input
                v-model="form.email"
                type="email"
                placeholder="请输入邮箱"
                :prefix-icon="Mail"
                class="h-12"
              />
            </el-form-item>

            <el-form-item class="mb-5">
              <label class="block text-sm font-medium text-gray-700 mb-2">密码</label>
              <el-input
                v-model="form.password"
                :type="showPassword ? 'text' : 'password'"
                placeholder="请输入密码（至少8个字符）"
                :prefix-icon="Lock"
                class="h-12 password-input"
              >
                <template #suffix>
                  <button
                    type="button"
                    class="w-8 h-8 inline-flex items-center justify-center rounded-lg text-gray-400 hover:text-[#0891b2] hover:bg-cyan-50 transition-colors"
                    @click.stop="showPassword = !showPassword"
                  >
                    <EyeOff v-if="showPassword" class="w-4 h-4" />
                    <Eye v-else class="w-4 h-4" />
                  </button>
                </template>
              </el-input>
            </el-form-item>

            <el-form-item class="mb-5">
              <label class="block text-sm font-medium text-gray-700 mb-2">确认密码</label>
              <el-input
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
                    @click.stop="showConfirmPassword = !showConfirmPassword"
                  >
                    <EyeOff v-if="showConfirmPassword" class="w-4 h-4" />
                    <Eye v-else class="w-4 h-4" />
                  </button>
                </template>
              </el-input>
            </el-form-item>

            <el-form-item class="mb-5">
              <label class="block text-sm font-medium text-gray-700 mb-2">邮箱验证码</label>
              <div class="flex gap-3">
                <el-input
                  v-model="form.code"
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

            <el-button type="primary" native-type="submit" class="w-full h-12 text-base font-semibold" :loading="loading">
              注册
            </el-button>
          </el-form>

          <p class="text-center text-gray-500 text-sm mt-8">
            已有账号？<router-link :to="loginRoute" class="text-[#0891b2] hover:text-[#0e7490] font-medium transition-colors">立即登录</router-link>
          </p>
        </div>
      </div>
    </div>
  </div>

  <SendCodeModal
    :visible="showSendCodeModal"
    :email="form.email"
    @close="showSendCodeModal = false"
    @success="handleSendCodeSuccess"
  />
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Mail, Lock, Eye, EyeOff, MessageSquare } from 'lucide-vue-next'
import { authAPI } from '../api/auth'
import SendCodeModal from '../components/SendCodeModal.vue'
import { getLoginRedirect, loadTargetClientName } from '../utils/oauthTarget'

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

const form = ref({
  email: '',
  password: '',
  confirmPassword: '',
  code: ''
})

const targetPlatformName = computed(() => targetClientName.value || 'Lite SSO')
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

const openSendCodeModal = () => {
  if (!form.value.email) {
    errorMessage.value = '请先输入邮箱'
    return
  }
  showSendCodeModal.value = true
}

const handleSendCodeSuccess = () => {
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

    if (form.value.password.length < 8) {
      errorMessage.value = '密码长度至少为8个字符'
      return
    }

    loading.value = true
    errorMessage.value = ''
    successMessage.value = ''

    await authAPI.register({
      email: form.value.email,
      password: form.value.password,
      otp: form.value.code
    })

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

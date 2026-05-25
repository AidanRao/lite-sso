<template>
  <div class="min-h-screen flex overflow-hidden">
    <div class="hidden lg:flex flex-[1.2] bg-gradient-to-br from-[#0f766e] via-[#0891b2] to-[#155e75] items-center justify-center p-12 relative overflow-hidden">
      <div class="absolute inset-0 bg-radial-gradient-circle opacity-15"></div>
      <div class="relative z-10 text-center text-white">
        <div class="inline-flex items-center justify-center w-16 h-16 bg-white/20 rounded-2xl mb-6">
          <KeyRound class="w-8 h-8" />
        </div>
        <h1 class="text-5xl font-bold mb-4 tracking-tight">重置密码</h1>
        <p class="text-lg opacity-90 max-w-md leading-relaxed">
          通过邮箱验证码确认身份后，设置一个新的登录密码。
        </p>
        <div class="mt-12 flex justify-center">
          <img
            src="/assets/images/default.png"
            alt="SSO Illustration"
            class="w-72 h-72 rounded-2xl shadow-2xl object-cover"
          />
        </div>
      </div>
    </div>

    <div class="flex-1 bg-gradient-to-br from-[#ecfeff] to-[#f0fdfa] flex items-center justify-center p-8">
      <div class="w-full max-w-md">
        <div class="bg-white rounded-2xl shadow-xl shadow-[#0891b2]/5 p-8 sm:p-10">
          <div class="mb-8">
            <router-link to="/login" class="inline-flex items-center gap-2 text-sm font-medium text-gray-500 hover:text-[#0891b2] transition-colors mb-6">
              <ArrowLeft class="w-4 h-4" />
              返回登录
            </router-link>
            <h2 class="text-2xl font-bold text-gray-800">找回密码</h2>
            <p class="text-gray-500 text-sm mt-2">输入邮箱验证码并设置新密码</p>
          </div>

          <div v-if="errorMessage" class="mb-4 p-4 bg-red-50 border border-red-200 rounded-xl text-red-600 text-sm">
            {{ errorMessage }}
          </div>
          <div v-if="successMessage" class="mb-4 p-4 bg-green-50 border border-green-200 rounded-xl text-green-600 text-sm">
            {{ successMessage }}
          </div>

          <el-form :model="form" @submit.prevent="handleReset" class="space-y-5">
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
              <label class="block text-sm font-medium text-gray-700 mb-2">邮箱验证码</label>
              <div class="flex gap-3">
                <el-input
                  v-model="form.otp"
                  type="text"
                  placeholder="请输入验证码"
                  maxlength="6"
                  :prefix-icon="MessageSquare"
                  class="h-12 flex-1"
                />
                <el-button
                  type="default"
                  @click="openSendCodeModal"
                  :disabled="countdown > 0"
                  class="h-12 px-5"
                >
                  {{ countdown > 0 ? `${countdown}秒` : '获取验证码' }}
                </el-button>
              </div>
            </el-form-item>

            <el-form-item class="mb-5">
              <label class="block text-sm font-medium text-gray-700 mb-2">新密码</label>
              <el-input
                v-model="form.password"
                :type="showPassword ? 'text' : 'password'"
                placeholder="请输入新密码（至少8位）"
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

            <el-form-item class="mb-6">
              <label class="block text-sm font-medium text-gray-700 mb-2">再次输入新密码</label>
              <el-input
                v-model="form.confirmPassword"
                :type="showConfirmPassword ? 'text' : 'password'"
                placeholder="请再次输入新密码"
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

            <button
              type="submit"
              class="w-full h-12 bg-[#0891b2] text-white rounded-xl font-semibold text-base hover:bg-[#0e7490] transition-colors flex items-center justify-center disabled:opacity-60"
              :disabled="loading"
            >
              {{ loading ? '提交中...' : '重置密码' }}
            </button>
          </el-form>
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
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Eye, EyeOff, KeyRound, Lock, Mail, MessageSquare } from 'lucide-vue-next'
import { authAPI } from '../api/auth'
import SendCodeModal from '../components/SendCodeModal.vue'

const router = useRouter()
const loading = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const showPassword = ref(false)
const showConfirmPassword = ref(false)
const showSendCodeModal = ref(false)
const countdown = ref(0)

const form = ref({
  email: '',
  otp: '',
  password: '',
  confirmPassword: ''
})

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

const handleReset = async () => {
  try {
    if (!form.value.email || !form.value.otp) {
      errorMessage.value = '请填写邮箱和验证码'
      return
    }
    if (form.value.password.length < 8) {
      errorMessage.value = '密码长度至少为8个字符'
      return
    }
    if (form.value.password !== form.value.confirmPassword) {
      errorMessage.value = '两次输入的密码不一致'
      return
    }

    loading.value = true
    errorMessage.value = ''
    successMessage.value = ''

    await authAPI.resetPassword({
      email: form.value.email,
      otp: form.value.otp,
      password: form.value.password
    })

    successMessage.value = '密码已重置，正在返回登录页...'
    setTimeout(() => {
      router.push('/login')
    }, 1000)
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loading.value = false
  }
}
</script>

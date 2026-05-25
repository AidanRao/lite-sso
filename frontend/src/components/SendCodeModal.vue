<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="visible" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/50 backdrop-blur-sm" @click="handleClose"></div>
        
        <div class="relative bg-white rounded-2xl shadow-2xl w-full max-w-md overflow-hidden transform transition-all">
          <div class="bg-gradient-to-r from-[#0891b2] to-[#0e7490] px-6 py-5">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <div class="w-10 h-10 bg-white/20 rounded-xl flex items-center justify-center">
                  <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                  </svg>
                </div>
                <div>
                  <h3 class="text-white font-semibold text-lg">获取验证码</h3>
                </div>
              </div>
              <button 
                @click="handleClose" 
                class="w-8 h-8 bg-white/20 hover:bg-white/30 rounded-lg flex items-center justify-center transition-colors"
              >
                <svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          <div class="p-6">
            <div v-if="errorMessage" class="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm flex items-center gap-2">
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              {{ errorMessage }}
            </div>

            <div class="mb-6">
              <label class="block text-sm font-medium text-gray-700 mb-3">图形验证码</label>
              <div class="flex items-center gap-3">
                <el-input
                  v-model="captchaInput"
                  type="text"
                  placeholder="请输入右图中的四位数字"
                  maxlength="4"
                  class="h-12 flex-1"
                  @keyup.enter="handleSubmit"
                />
                <img
                  :src="captchaBase64"
                  alt="验证码"
                  class="h-12 w-28 cursor-pointer rounded-lg object-contain bg-gray-50 hover:bg-gray-100 transition-colors"
                  @click="refreshCaptcha"
                />
              </div>
              <p class="text-xs text-gray-400 mt-2">点击图片刷新</p>
            </div>

            <div class="flex gap-3">
              <el-button 
                type="default" 
                class="flex-1 h-11" 
                @click="handleClose"
              >
                取消
              </el-button>
              <el-button 
                type="primary" 
                class="flex-1 h-11 font-semibold" 
                @click="handleSubmit"
                :loading="loading"
              >
                发送
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import { authAPI } from '../api/auth'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false
  },
  email: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['close', 'success'])

const captchaBase64 = ref('')
const captchaInput = ref('')
const captchaID = ref('')
const loading = ref(false)
const errorMessage = ref('')

const loadCaptcha = async () => {
  try {
    const result = await authAPI.getCaptcha()
    const response = result.data || result
    
    if (!response || !response.captcha_png_base64 || !response.captcha_png_base64.trim()) {
      throw new Error('验证码图片为空')
    }
    
    captchaBase64.value = response.captcha_png_base64
    captchaID.value = response.captcha_id
    
    captchaInput.value = ''
    errorMessage.value = ''
  } catch (error) {
    console.error('获取验证码失败:', error)
    errorMessage.value = error.message || '获取验证码失败，请重试'
  }
}

const refreshCaptcha = () => {
  loadCaptcha()
}

const handleSubmit = async () => {
  if (!captchaInput.value || captchaInput.value.length !== 4) {
    errorMessage.value = '请输入正确的验证码'
    return
  }

  if (!props.email) {
    errorMessage.value = '请先填写邮箱'
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    await authAPI.sendEmailCode({
      email: props.email,
      captcha_id: captchaID.value,
      captcha: captchaInput.value
    })

    emit('success')
    handleClose()
  } catch (error) {
    errorMessage.value = error.message
    loadCaptcha()
  } finally {
    loading.value = false
  }
}

const handleClose = () => {
  emit('close')
}

watch(() => props.visible, (newVal) => {
  if (newVal) {
    loadCaptcha()
  }
})

onMounted(() => {
  if (props.visible) {
    loadCaptcha()
  }
})
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: all 0.3s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from .relative,
.modal-leave-to .relative {
  transform: scale(0.9) translateY(20px);
}
</style>

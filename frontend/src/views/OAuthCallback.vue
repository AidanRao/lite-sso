<template>
  <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-[#ecfeff] to-[#f0fdfa]">
    <div class="text-center">
      <div v-if="loading" class="flex flex-col items-center">
        <svg class="animate-spin h-12 w-12 text-[#0891b2] mb-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
        <p class="text-gray-600">正在处理登录...</p>
      </div>
      <div v-else-if="error" class="flex flex-col items-center">
        <svg class="w-12 h-12 text-red-500 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <p class="text-red-600 mb-4">{{ error }}</p>
        <a href="/login" class="text-[#0891b2] hover:text-[#0e7490] font-medium">返回登录页面</a>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const loading = ref(true)
const error = ref('')

onMounted(async () => {
  const params = new URLSearchParams(window.location.search)
  const errorParam = params.get('error')

  if (errorParam) {
    loading.value = false
    error.value = '登录失败：' + decodeURIComponent(errorParam)
    return
  }

  window.location.replace('/profile')
})
</script>

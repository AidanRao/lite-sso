<template>
  <main class="logout-page">
    <section class="logout-status" aria-live="polite" :aria-busy="pending">
      <LogOut v-if="!pending && !errorMessage" class="logout-icon" :size="36" aria-hidden="true" />
      <LoaderCircle v-else-if="pending" class="logout-spinner" :size="36" aria-hidden="true" />
      <CircleAlert v-else class="logout-error" :size="36" aria-hidden="true" />
      <h1>{{ errorMessage ? '退出失败' : pending ? '正在退出登录' : '是否退出登录？' }}</h1>
      <p>{{ errorMessage || (pending ? '请稍候，正在安全退出当前账号…' : '退出后，您需要重新登录才能访问账号及已登录的应用。') }}</p>
      <div v-if="!pending" class="logout-actions">
        <RouterLink class="cancel-button" to="/profile">{{ errorMessage ? '返回账号' : '取消' }}</RouterLink>
        <button type="button" @click="logout">{{ errorMessage ? '重试' : '确认退出' }}</button>
      </div>
    </section>
    <iframe
      v-for="(uri, index) in logoutURIs"
      :key="index"
      :src="uri"
      title="应用退出通知"
      hidden
      @load="completeFrame(index)"
      @error="completeFrame(index)"
    />
  </main>
</template>

<script setup>
import { onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { CircleAlert, LoaderCircle, LogOut } from 'lucide-vue-next'
import { authAPI, clearAccessToken } from '../api/auth'

const route = useRoute()
const errorMessage = ref('')
const pending = ref(false)
const logoutURIs = ref([])
const completedFrames = new Set()
let redirectURI = '/login'
let redirectTimer
let timeoutTimer
let controller
let disposed = false
let redirected = false

const redirect = () => {
  if (disposed || redirected) return
  redirected = true
  clearTimeout(redirectTimer)
  clearTimeout(timeoutTimer)
  window.location.replace(redirectURI)
}

const completeFrame = (index) => {
  if (disposed || completedFrames.has(index)) return
  completedFrames.add(index)
  if (completedFrames.size === logoutURIs.value.length) {
    redirectTimer = setTimeout(redirect, 500)
  }
}

const logout = async () => {
  if (pending.value) return
  pending.value = true
  errorMessage.value = ''
  controller = new AbortController()
  try {
    const response = await authAPI.logout(typeof route.query.redirect === 'string' ? route.query.redirect : '/login', controller.signal)
    if (disposed) return
    const data = response.data
    if (!data?.logged_out) throw new Error('退出失败，请重试')
    clearAccessToken()
    redirectURI = data.redirect_uri || '/login'
    logoutURIs.value = data.logout_uris || []
    if (!logoutURIs.value.length) {
      redirect()
      return
    }
    timeoutTimer = setTimeout(redirect, 5000)
  } catch (error) {
    if (disposed) return
    if (error.response?.status === 401) {
      clearAccessToken()
      redirectURI = '/login'
      redirect()
      return
    }
    pending.value = false
    errorMessage.value = error.response?.data?.message || '暂时无法退出登录，请重试'
  }
}

onUnmounted(() => {
  disposed = true
  controller?.abort()
  clearTimeout(redirectTimer)
  clearTimeout(timeoutTimer)
})
</script>

<style scoped>
.logout-page {
  min-height: 100vh;
  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: #fff;
}
.logout-status { max-width: 360px; text-align: center; }
.logout-icon, .logout-spinner, .logout-error { margin: 0 auto 20px; color: #0891b2; }
.logout-spinner { animation: spin 1s linear infinite; }
.logout-error { color: #dc2626; }
h1 { font-size: 24px; font-weight: 600; color: #1f2937; }
p { margin-top: 12px; font-size: 14px; color: #6b7280; }
.logout-actions { display: flex; justify-content: center; gap: 12px; margin-top: 24px; }
.cancel-button { padding: 10px 28px; border-radius: 10px; background: #f3f4f6; color: #374151; }
button { padding: 10px 28px; border-radius: 10px; background: #0891b2; color: #fff; }
button:focus-visible, .cancel-button:focus-visible { outline: 2px solid #0891b2; outline-offset: 3px; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .logout-spinner { animation: none; } }
</style>

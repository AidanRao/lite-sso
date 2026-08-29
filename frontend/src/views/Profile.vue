<template>
  <main class="settings-page">
    <div class="settings-shell">
      <ProfileSidebar :user="user" :is-admin="isAdmin" />

      <section class="settings-content" aria-live="polite">
        <div v-if="loading" class="page-state">
          <span class="spinner" aria-hidden="true"></span>
          <p>正在加载账号设置…</p>
        </div>

        <div v-else-if="loadError" class="page-state error-state">
          <CircleAlert :size="24" aria-hidden="true" />
          <p>{{ loadError }}</p>
          <button class="button" type="button" @click="loadProfile">重新加载</button>
        </div>

        <RouterView v-else />
      </section>
    </div>
  </main>
</template>

<script setup>
import { provide, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CircleAlert } from 'lucide-vue-next'
import { userAPI } from '../api/auth'
import ProfileSidebar from '../components/profile/ProfileSidebar.vue'
import { PROFILE_CONTEXT_KEY } from './profile/profileContext'

const route = useRoute()
const router = useRouter()
const user = ref(null)
const isAdmin = ref(false)
const loading = ref(true)
const loadError = ref('')

const setUser = (nextUser) => {
  if (nextUser) user.value = nextUser
}

const loadProfile = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const result = await userAPI.getProfile()
    const data = result?.data || {}
    user.value = data.user || null
    isAdmin.value = Boolean(data.is_admin)
  } catch (error) {
    if (error.status === 401) {
      router.replace({ path: '/login', query: { redirect: route.fullPath } })
      return
    }
    loadError.value = error.message || '账号设置加载失败'
  } finally {
    loading.value = false
  }
}

provide(PROFILE_CONTEXT_KEY, {
  user,
  isAdmin,
  setUser,
  reload: loadProfile
})

loadProfile()
</script>

<style scoped>
.settings-page {
  min-height: 100vh;
  box-sizing: border-box;
  background: #ffffff;
  color: #24292f;
  padding: 28px 24px 56px;
}

.settings-shell {
  display: flex;
  width: min(1280px, 100%);
  align-items: flex-start;
  gap: 42px;
  margin: 0 auto;
}

.settings-content {
  min-width: 0;
  flex: 1 1 auto;
}

.page-state {
  display: flex;
  min-height: 280px;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border: 1px solid #d0d7de;
  border-radius: 6px;
  color: #57606a;
}

.page-state p {
  margin: 0;
}

.error-state {
  flex-direction: column;
  color: #cf222e;
}

.spinner {
  width: 18px;
  height: 18px;
  border: 2px solid #d0d7de;
  border-top-color: #0969da;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.button {
  min-height: 32px;
  border: 1px solid rgba(27, 31, 36, 0.15);
  border-radius: 6px;
  background: #f6f8fa;
  color: #24292f;
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  padding: 5px 12px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 900px) {
  .settings-shell {
    gap: 24px;
  }
}

@media (max-width: 760px) {
  .settings-page {
    padding: 20px 16px 40px;
  }

  .settings-shell {
    display: grid;
    gap: 28px;
  }
}
</style>

<template>
  <div class="settings-view">
    <ProfileSettingsSection title-id="applications-title" title="已登录应用">
      <div v-if="loading" class="section-state">
        <span class="spinner" aria-hidden="true"></span>
        正在加载应用…
      </div>
      <div v-else-if="errorMessage" class="section-state error-state">
        <span>{{ errorMessage }}</span>
        <button class="button" type="button" @click="loadApplications">重新加载</button>
      </div>
      <div v-else-if="applications.length" class="applications-list">
        <div class="applications-head" aria-hidden="true">
          <span>应用</span>
          <span>Client ID</span>
          <span>最近登录</span>
        </div>
        <article v-for="application in applications" :key="application.client_id" class="application-row">
          <div class="application-identity">
            <ApplicationLogo :label="application.name || application.client_id" :src="application.logo_url" size="small" />
            <div class="application-copy">
              <a
                v-if="application.homepage_url"
                :href="application.homepage_url"
                target="_blank"
                rel="noopener noreferrer"
              >
                {{ application.name || application.client_id }}
                <ExternalLink :size="13" aria-hidden="true" />
              </a>
              <strong v-else>{{ application.name || application.client_id }}</strong>
            </div>
          </div>
          <div class="application-field">
            <span class="mobile-label">Client ID</span>
            <code>{{ application.client_id }}</code>
          </div>
          <div class="application-field">
            <span class="mobile-label">最近登录</span>
            <time>{{ formatDate(application.last_login_at) }}</time>
          </div>
        </article>
      </div>
      <div v-else class="section-state">暂无登录应用。</div>
    </ProfileSettingsSection>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ExternalLink } from 'lucide-vue-next'
import { userAPI } from '../../api/auth'
import ApplicationLogo from '../../components/ApplicationLogo.vue'
import ProfileSettingsSection from '../../components/profile/ProfileSettingsSection.vue'

const route = useRoute()
const router = useRouter()
const applications = ref([])
const loading = ref(true)
const errorMessage = ref('')

const loadApplications = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    const result = await userAPI.getApplications()
    applications.value = Array.isArray(result?.data?.applications) ? result.data.applications : []
  } catch (error) {
    if (error.status === 401) {
      router.replace({ path: '/login', query: { redirect: route.fullPath } })
      return
    }
    applications.value = []
    errorMessage.value = error.message || '应用加载失败'
  } finally {
    loading.value = false
  }
}

const formatDate = (value) => {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'
  }).format(new Date(value))
}

onMounted(loadApplications)
</script>

<style scoped>
.settings-view {
  display: grid;
  gap: 22px;
}

.applications-list {
  overflow: hidden;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
}

.applications-head,
.application-row {
  display: grid;
  grid-template-columns: minmax(220px, 1.25fr) minmax(160px, 0.8fr) minmax(150px, 0.65fr);
  align-items: center;
  gap: 18px;
  padding: 12px 16px;
}

.applications-head {
  background: var(--profile-surface-subtle);
  color: var(--profile-text-muted);
  font-size: 12px;
  font-weight: 600;
}

.application-row {
  min-height: 66px;
  border-top: 1px solid var(--profile-divider);
}

.application-identity {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 12px;
}

.application-copy {
  min-width: 0;
}

.application-copy a,
.application-copy strong {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 5px;
  color: var(--profile-text-strong);
  font-size: 14px;
  font-weight: 600;
}

.application-copy a {
  color: var(--profile-accent);
  text-decoration: none;
}

.application-copy a:hover {
  text-decoration: underline;
}

.application-field {
  min-width: 0;
}

.application-field code,
.application-field time {
  overflow: hidden;
  color: var(--profile-text-muted);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-label {
  display: none;
}

.section-state {
  display: flex;
  min-height: 110px;
  box-sizing: border-box;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
  color: var(--profile-text-muted);
  font-size: 14px;
  padding: 16px;
}

.error-state {
  justify-content: space-between;
  color: var(--profile-danger);
}

.spinner {
  width: 17px;
  height: 17px;
  border: 2px solid var(--profile-border);
  border-top-color: var(--profile-accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.button {
  min-height: 32px;
  border: 1px solid var(--profile-border-muted);
  border-radius: 6px;
  background: var(--profile-surface-subtle);
  color: var(--profile-text-strong);
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  padding: 5px 12px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 700px) {
  .applications-head {
    display: none;
  }

  .application-row {
    display: grid;
    min-height: auto;
    grid-template-columns: 1fr;
    gap: 14px;
    padding: 16px;
  }

  .application-row:first-child {
    border-top: 0;
  }

  .application-field {
    display: grid;
    gap: 4px;
  }

  .mobile-label {
    display: block;
    color: var(--profile-text-faint);
    font-size: 12px;
    font-weight: 600;
  }
}
</style>

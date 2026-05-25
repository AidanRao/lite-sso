<template>
  <main class="profile-page">
    <section class="profile-hero">
      <div class="identity">
        <div class="avatar">
          <img v-if="user?.avatar_url" :src="user.avatar_url" :alt="displayName" />
          <span v-else>{{ avatarInitial }}</span>
        </div>
        <div class="identity-text">
          <p class="eyebrow">账户中心</p>
          <h1>{{ displayName }}</h1>
          <div class="identity-meta">
            <span>
              <Mail :size="16" />
              {{ displayEmail }}
            </span>
            <span>
              <ShieldCheck :size="16" />
              已登录
            </span>
          </div>
        </div>
      </div>
      <div class="hero-actions">
        <button class="icon-button" type="button" title="刷新" @click="loadProfile">
          <RefreshCw :size="18" />
        </button>
        <button class="secondary-button" type="button" @click="logout">
          <LogOut :size="18" />
          退出登录
        </button>
      </div>
    </section>

    <section class="content-grid">
      <article class="panel user-panel">
        <div class="panel-heading">
          <div>
            <p class="section-kicker">Profile</p>
            <h2>用户信息</h2>
          </div>
          <UserRound :size="22" />
        </div>

        <div class="info-list">
          <div class="info-row">
            <span>用户 ID</span>
            <strong class="mono">{{ user?.id || '-' }}</strong>
          </div>
          <div class="info-row">
            <span>用户名</span>
            <strong>{{ user?.username || '未设置' }}</strong>
          </div>
          <div class="info-row">
            <span>邮箱</span>
            <strong>{{ user?.email || '未设置' }}</strong>
          </div>
          <div class="info-row">
            <span>头像</span>
            <strong class="truncate-value">{{ user?.avatar_url || '未设置' }}</strong>
          </div>
        </div>
      </article>

      <article class="panel provider-panel">
        <div class="panel-heading">
          <div>
            <p class="section-kicker">Connections</p>
            <h2>第三方登录方式</h2>
          </div>
          <Link2 :size="22" />
        </div>

        <div class="provider-list">
          <div v-for="provider in providerCards" :key="provider.id" class="provider-row">
            <div class="provider-mark" :class="provider.id">
              <Github v-if="provider.id === 'github'" :size="22" />
              <span v-else>飞</span>
            </div>
            <div class="provider-copy">
              <strong>{{ provider.name }}</strong>
              <span>{{ provider.bound ? '已绑定到当前账号' : '未绑定' }}</span>
            </div>
            <span class="status-pill" :class="{ bound: provider.bound }">
              <CheckCircle2 v-if="provider.bound" :size="15" />
              <CircleAlert v-else :size="15" />
              {{ provider.bound ? '已绑定' : '未绑定' }}
            </span>
            <button
              v-if="!provider.bound"
              class="bind-button"
              type="button"
              @click="bindProvider(provider.id)"
            >
              绑定
            </button>
          </div>
        </div>
      </article>

      <article class="panel apps-panel">
        <div class="panel-heading">
          <div>
            <p class="section-kicker">Applications</p>
            <h2>当前账号下登录的应用</h2>
          </div>
          <BriefcaseBusiness :size="22" />
        </div>

        <div v-if="applications.length" class="app-list">
          <div v-for="app in applications" :key="app.client_id" class="app-row">
            <div class="app-icon">
              <MonitorSmartphone :size="22" />
            </div>
            <div class="app-copy">
              <strong>{{ app.name || app.client_id }}</strong>
              <span>Client ID: {{ app.client_id }}</span>
            </div>
            <time>{{ formatDate(app.last_login_at) }}</time>
          </div>
        </div>

        <div v-else class="empty-state">
          <MonitorSmartphone :size="26" />
          <div>
            <strong>暂无应用登录记录</strong>
            <span>通过 OAuth 授权进入业务应用后，这里会显示最近登录的应用。</span>
          </div>
        </div>
      </article>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  BriefcaseBusiness,
  CheckCircle2,
  CircleAlert,
  Github,
  Link2,
  LogOut,
  Mail,
  MonitorSmartphone,
  RefreshCw,
  ShieldCheck,
  UserRound
} from 'lucide-vue-next'

const router = useRouter()
const route = useRoute()
const user = ref(null)
const applications = ref([])
const thirdPartyProviders = ref([])

const providerMeta = [
  { id: 'github', name: 'GitHub' },
  { id: 'feishu', name: '飞书' }
]

const displayName = computed(() => user.value?.username || user.value?.email || 'SSO 用户')
const displayEmail = computed(() => user.value?.email || '邮箱未设置')
const avatarInitial = computed(() => displayName.value.slice(0, 1).toUpperCase())

const providerCards = computed(() => {
  const bindings = new Map(thirdPartyProviders.value.map((item) => [item.provider, item.bound]))
  return providerMeta.map((provider) => ({
    ...provider,
    bound: Boolean(bindings.get(provider.id))
  }))
})

const loadProfile = async () => {
  try {
    const response = await fetch('/api/user/profile')
    if (response.status === 401) {
      router.push('/login?redirect=/profile')
      return
    }
    if (!response.ok) {
      throw new Error('获取资料失败')
    }

    const result = await response.json()
    const data = result?.data || {}
    user.value = data.user || null
    applications.value = Array.isArray(data.applications) ? data.applications : []
    thirdPartyProviders.value = Array.isArray(data.third_party_providers) ? data.third_party_providers : []
  } catch (error) {
    ElMessage.error(error.message || '获取资料失败')
  }
}

const logout = async () => {
  try {
    await fetch('/api/auth/logout', {
      method: 'POST'
    })
  } catch (error) {
    ElMessage.error(error.message || '退出失败')
  }
  router.push('/login')
}

const bindProvider = (provider) => {
  const redirect = encodeURIComponent('/profile?bind=success')
  window.location.href = `/api/user/third/${provider}/bind?redirect=${redirect}`
}

const formatDate = (value) => {
  if (!value) {
    return '未知时间'
  }
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value))
}

onMounted(() => {
  if (route.query.bind === 'success') {
    ElMessage.success('第三方账号绑定成功')
    router.replace('/profile')
  } else if (route.query.bind_error) {
    ElMessage.error(String(route.query.bind_error))
    router.replace('/profile')
  }
  loadProfile()
})
</script>

<style scoped>
.profile-page {
  min-height: 100vh;
  padding: 40px 24px;
  background:
    linear-gradient(180deg, rgba(241, 245, 249, 0.92), rgba(236, 253, 245, 0.78)),
    #f8fafc;
  color: #172033;
}

.profile-hero,
.content-grid {
  width: min(1120px, 100%);
  margin: 0 auto;
}

.profile-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 8px 0 30px;
}

.identity {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 20px;
}

.avatar {
  width: 88px;
  height: 88px;
  flex: 0 0 88px;
  overflow: hidden;
  border-radius: 24px;
  background: linear-gradient(135deg, #0f766e, #2563eb);
  box-shadow: 0 18px 40px rgba(15, 118, 110, 0.18);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 34px;
  font-weight: 800;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.identity-text {
  min-width: 0;
}

.eyebrow,
.section-kicker {
  margin: 0;
  color: #0f766e;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0;
  text-transform: uppercase;
}

.identity-text h1 {
  margin: 7px 0 10px;
  color: #111827;
  font-size: 34px;
  line-height: 1.1;
}

.identity-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.identity-meta span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 30px;
  padding: 0 11px;
  border: 1px solid #dbe5ed;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.72);
  color: #475569;
  font-size: 13px;
}

.hero-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

button {
  border: 0;
  cursor: pointer;
  font: inherit;
}

.icon-button,
.secondary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #d9e4ec;
  background: #fff;
  color: #334155;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.06);
  transition: transform 0.2s ease, border-color 0.2s ease, color 0.2s ease;
}

.icon-button {
  width: 44px;
  height: 44px;
  border-radius: 12px;
}

.secondary-button {
  height: 44px;
  gap: 8px;
  padding: 0 16px;
  border-radius: 12px;
  font-weight: 700;
}

.icon-button:hover,
.secondary-button:hover {
  transform: translateY(-1px);
  border-color: #0f766e;
  color: #0f766e;
}

.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(320px, 0.92fr);
  gap: 18px;
}

.panel {
  min-width: 0;
  border: 1px solid rgba(203, 213, 225, 0.78);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 20px 45px rgba(15, 23, 42, 0.06);
  padding: 24px;
}

.apps-panel {
  grid-column: 1 / -1;
}

.panel-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
  color: #0f766e;
}

.panel-heading h2 {
  margin: 5px 0 0;
  color: #111827;
  font-size: 20px;
}

.info-list,
.provider-list,
.app-list {
  display: grid;
  gap: 12px;
}

.info-row,
.provider-row,
.app-row {
  display: grid;
  align-items: center;
  min-width: 0;
  border-top: 1px solid #edf2f7;
  padding-top: 12px;
}

.info-row {
  grid-template-columns: 116px minmax(0, 1fr);
  gap: 18px;
}

.info-row span,
.provider-copy span,
.app-copy span,
.empty-state span {
  color: #64748b;
  font-size: 14px;
}

.info-row strong,
.provider-copy strong,
.app-copy strong,
.empty-state strong {
  min-width: 0;
  color: #111827;
  font-weight: 750;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
}

.truncate-value {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-row {
  grid-template-columns: 44px minmax(0, 1fr) auto auto;
  gap: 12px;
}

.provider-mark,
.app-icon {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.provider-mark.github {
  background: #111827;
  color: #fff;
}

.provider-mark.feishu {
  background: #e0f2fe;
  color: #0369a1;
  font-weight: 800;
}

.provider-copy,
.app-copy {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  min-width: 74px;
  height: 30px;
  padding: 0 10px;
  border-radius: 999px;
  background: #f8fafc;
  color: #64748b;
  font-size: 13px;
  font-weight: 700;
}

.status-pill.bound {
  background: #ecfdf5;
  color: #047857;
}

.bind-button {
  height: 34px;
  min-width: 62px;
  justify-self: start;
  padding: 0 14px;
  border-radius: 8px;
  background: #0f766e;
  color: #fff;
  font-size: 14px;
  font-weight: 750;
  transition: transform 0.2s ease, background 0.2s ease;
}

.bind-button:hover {
  transform: translateY(-1px);
  background: #115e59;
}

.app-row {
  grid-template-columns: 48px minmax(0, 1fr) 120px;
  gap: 14px;
}

.app-icon {
  background: #fff7ed;
  color: #c2410c;
}

.app-row time {
  justify-self: end;
  color: #475569;
  font-size: 14px;
  font-weight: 700;
}

.empty-state {
  display: flex;
  align-items: center;
  gap: 14px;
  border-top: 1px solid #edf2f7;
  padding-top: 18px;
  color: #0f766e;
}

.empty-state div {
  display: grid;
  gap: 4px;
}

@media (max-width: 820px) {
  .profile-page {
    padding: 28px 16px;
  }

  .profile-hero {
    align-items: flex-start;
    flex-direction: column;
  }

  .content-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .identity {
    align-items: flex-start;
  }

  .avatar {
    width: 64px;
    height: 64px;
    flex-basis: 64px;
    border-radius: 18px;
    font-size: 26px;
  }

  .identity-text h1 {
    font-size: 26px;
  }

  .hero-actions {
    width: 100%;
  }

  .secondary-button {
    flex: 1;
  }

  .panel {
    padding: 18px;
  }

  .info-row,
  .provider-row,
  .app-row {
    grid-template-columns: 1fr;
    align-items: flex-start;
  }

  .provider-row,
  .app-row {
    position: relative;
    padding-left: 56px;
  }

  .provider-mark,
  .app-icon {
    position: absolute;
    left: 0;
    top: 12px;
  }

  .status-pill,
  .app-row time {
    justify-self: start;
  }
}
</style>

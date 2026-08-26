<template>
  <main class="profile-page">
    <section class="account-hero">
      <div class="account-summary">
        <div class="avatar">
          <img v-if="user?.avatar_url" :src="user.avatar_url" :alt="displayName" />
          <span v-else>{{ avatarInitial }}</span>
        </div>
        <div class="account-copy">
          <h1>{{ displayName }}</h1>
          <p>{{ displayEmail }}</p>
        </div>
      </div>

      <div class="account-actions">
        <button v-if="isAdmin" class="admin-button" type="button" @click="router.push('/admin')">管理后台</button>
        <button class="logout-button" type="button" @click="logout">退出登录</button>
      </div>
    </section>

    <section class="profile-grid">
      <article class="panel account-panel">
        <header class="panel-header">
          <h2>账号</h2>
        </header>

        <div class="field-list">
          <div class="field-row">
            <span>用户 ID</span>
            <div class="id-value">
              <strong class="mono">{{ user?.id || '-' }}</strong>
              <button class="icon-button" type="button" title="复制用户 ID" :disabled="!user?.id" @click="copyUserID">
                <Check v-if="idCopied" :size="17" />
                <Copy v-else :size="17" />
              </button>
            </div>
          </div>
          <div class="field-row">
            <span>用户名</span>
            <div class="username-value">
              <strong>{{ user?.username || '未设置' }}</strong>
              <button class="icon-button" type="button" title="修改用户名" :disabled="!user" @click="openUsernameDialog">
                <Pencil :size="17" />
              </button>
            </div>
          </div>
          <div class="field-row">
            <span>邮箱</span>
            <strong>{{ user?.email || '未设置' }}</strong>
          </div>
        </div>
      </article>

      <article class="panel provider-panel">
        <header class="panel-header">
          <h2>登录方式</h2>
        </header>

        <div class="provider-list">
          <div v-for="provider in providerCards" :key="provider.id" class="provider-row">
            <div>
              <strong>{{ provider.name }}</strong>
              <span>{{ provider.bound ? '已绑定' : '未绑定' }}</span>
            </div>
            <button
              v-if="!provider.bound"
              class="bind-button"
              type="button"
              @click="bindProvider(provider.id)"
            >
              绑定
            </button>
            <button
              v-else
              class="unbind-button"
              type="button"
              :disabled="!user?.email || Boolean(unbindingProvider)"
              :title="user?.email ? `撤销 ${provider.name} 授权` : '请先设置邮箱，再撤销第三方授权'"
              @click="unbindProvider(provider)"
            >
              {{ unbindingProvider === provider.id ? '撤销中' : '撤销授权' }}
            </button>
          </div>
        </div>
      </article>

      <article class="panel devices-panel">
        <header class="panel-header">
          <h2>登录设备</h2>
          <span>{{ devices.length }} 台</span>
        </header>

        <div v-if="devicesLoading" class="empty-state">
          正在加载登录设备
        </div>

        <div v-else-if="devicesError" class="empty-state device-error-state">
          <span>{{ devicesError }}</span>
          <button class="retry-button" type="button" @click="loadDevices">重新加载</button>
        </div>

        <div v-else-if="deviceCards.length" class="device-list">
          <div v-for="device in deviceCards" :key="device.device_id" class="device-row">
            <div class="device-icon" aria-hidden="true">
              <Smartphone v-if="device.mobile" :size="20" />
              <Monitor v-else :size="20" />
            </div>

            <div class="device-content">
              <div class="device-title">
                <strong>{{ device.name }}</strong>
                <span v-if="device.current" class="current-device-badge">当前设备</span>
              </div>
              <span class="device-agent" :title="device.user_agent || '未知 User-Agent'">
                {{ device.user_agent || '未知 User-Agent' }}
              </span>
              <div class="device-meta">
                <span>IP {{ device.ip || '-' }}</span>
                <span>{{ authMethodLabel(device.auth_method) }}</span>
                <span>登录于 {{ formatDate(device.created_at) }}</span>
                <span>最近活动 {{ formatDate(device.last_seen_at) }}</span>
              </div>
            </div>

            <button
              v-if="!device.current"
              class="device-revoke-button"
              type="button"
              :disabled="Boolean(revokingDevice)"
              @click="revokeDevice(device)"
            >
              {{ revokingDevice === device.device_id ? '踢出中' : '踢出设备' }}
            </button>
          </div>
        </div>

        <div v-else class="empty-state">
          暂无登录设备
        </div>
      </article>

      <article class="panel apps-panel">
        <header class="panel-header">
          <h2>登录应用</h2>
          <span>{{ applications.length }} 个</span>
        </header>

        <div v-if="applications.length" class="app-table">
          <div class="app-row app-head">
            <span>应用</span>
            <span>Client ID</span>
            <span>最近登录</span>
          </div>
          <div v-for="app in applications" :key="app.client_id" class="app-row">
            <a
              v-if="app.homepage_url"
              class="app-link"
              :href="app.homepage_url"
              target="_blank"
              rel="noopener noreferrer"
            >
              {{ app.name || app.client_id }}
            </a>
            <strong v-else>{{ app.name || app.client_id }}</strong>
            <span class="mono">{{ app.client_id }}</span>
            <time>{{ formatDate(app.last_login_at) }}</time>
          </div>
        </div>

        <div v-else class="empty-state">
          暂无登录应用
        </div>
      </article>
    </section>

    <div v-if="usernameDialogOpen" class="dialog-mask" @click.self="closeUsernameDialog">
      <form class="dialog" @submit.prevent="saveUsername">
        <header>
          <h2>修改用户名</h2>
          <button class="icon-button" type="button" title="关闭" @click="closeUsernameDialog">
            <X :size="18" />
          </button>
        </header>

        <label>
          <span>用户名</span>
          <input ref="usernameInput" v-model="usernameDraft" maxlength="50" placeholder="未设置" />
        </label>

        <footer>
          <button class="text-button secondary" type="button" @click="closeUsernameDialog">取消</button>
          <button class="text-button primary" type="submit" :disabled="usernameSaving || !hasUsernameChanged">
            保存
          </button>
        </footer>
      </form>
    </div>
  </main>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Check, Copy, Monitor, Pencil, Smartphone, X } from 'lucide-vue-next'
import { userAPI } from '../api/auth'
import { submitGlobalLogout } from '../utils/logout'

const router = useRouter()
const route = useRoute()
const user = ref(null)
const applications = ref([])
const thirdPartyProviders = ref([])
const devices = ref([])
const devicesLoading = ref(true)
const devicesError = ref('')
const isAdmin = ref(false)
const usernameDraft = ref('')
const usernameSaving = ref(false)
const usernameDialogOpen = ref(false)
const usernameInput = ref(null)
const idCopied = ref(false)
const unbindingProvider = ref('')
const revokingDevice = ref('')
let copyTimer = null

const providerMeta = [
  { id: 'github', name: 'GitHub' },
  { id: 'feishu', name: '飞书' }
]

const displayName = computed(() => user.value?.username || user.value?.email || 'SSO 用户')
const displayEmail = computed(() => user.value?.email || '邮箱未设置')
const avatarInitial = computed(() => displayName.value.slice(0, 1).toUpperCase())
const hasUsernameChanged = computed(() => usernameDraft.value.trim() !== (user.value?.username || ''))

const providerCards = computed(() => {
  const bindings = new Map(thirdPartyProviders.value.map((item) => [item.provider, item.bound]))
  return providerMeta.map((provider) => ({
    ...provider,
    bound: Boolean(bindings.get(provider.id))
  }))
})

const deviceCards = computed(() => devices.value.map((device) => ({
  ...device,
  ...describeUserAgent(device.user_agent)
})))

const loadProfile = async () => {
  try {
    const result = await userAPI.getProfile()
    const data = result?.data || {}
    user.value = data.user || null
    usernameDraft.value = user.value?.username || ''
    applications.value = Array.isArray(data.applications) ? data.applications : []
    thirdPartyProviders.value = Array.isArray(data.third_party_providers) ? data.third_party_providers : []
    isAdmin.value = Boolean(data.is_admin)
  } catch (error) {
    if (error.status === 401) {
      router.push('/login?redirect=/profile')
      return
    }
    ElMessage.error(error.message || '获取资料失败')
  }
}

const loadDevices = async () => {
  devicesLoading.value = true
  devicesError.value = ''
  try {
    const result = await userAPI.getDevices()
    const data = result?.data || {}
    devices.value = Array.isArray(data.devices) ? data.devices : []
  } catch (error) {
    if (error.status === 401) {
      router.push('/login?redirect=/profile')
      return
    }
    devices.value = []
    devicesError.value = error.message || '登录设备加载失败'
    ElMessage.error(devicesError.value)
  } finally {
    devicesLoading.value = false
  }
}

const openUsernameDialog = async () => {
  usernameDraft.value = user.value?.username || ''
  usernameDialogOpen.value = true
  await nextTick()
  usernameInput.value?.focus()
}

const closeUsernameDialog = () => {
  if (usernameSaving.value) {
    return
  }
  usernameDraft.value = user.value?.username || ''
  usernameDialogOpen.value = false
}

const saveUsername = async () => {
  if (usernameSaving.value || !hasUsernameChanged.value) {
    return
  }

  usernameSaving.value = true
  try {
    const result = await userAPI.updateProfile({
      username: usernameDraft.value
    })
    user.value = result?.data?.user || user.value
    usernameDraft.value = user.value?.username || ''
    usernameDialogOpen.value = false
    ElMessage.success('用户名已更新')
  } catch (error) {
    if (error.status === 401) {
      router.push('/login?redirect=/profile')
      return
    }
    ElMessage.error(error.message || '更新失败')
  } finally {
    usernameSaving.value = false
  }
}

const copyUserID = async () => {
  if (!user.value?.id) {
    return
  }

  try {
    await navigator.clipboard.writeText(user.value.id)
    idCopied.value = true
    ElMessage.success('用户 ID 已复制')
    window.clearTimeout(copyTimer)
    copyTimer = window.setTimeout(() => {
      idCopied.value = false
    }, 1400)
  } catch (error) {
    ElMessage.error(error.message || '复制失败')
  }
}

const logout = () => {
  submitGlobalLogout('/login')
}

const bindProvider = (provider) => {
  const redirect = encodeURIComponent('/profile?bind=success')
  window.location.href = `/api/user/third/${provider}/bind?redirect=${redirect}`
}

const unbindProvider = async (provider) => {
  if (!user.value?.email || unbindingProvider.value) {
    return
  }

  try {
    await ElMessageBox.confirm(
      `撤销后将无法再使用 ${provider.name} 登录当前账号，确定继续吗？`,
      `撤销 ${provider.name} 授权`,
      {
        confirmButtonText: '撤销授权',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
  } catch {
    return
  }

  unbindingProvider.value = provider.id
  try {
    await userAPI.unbindThirdParty(provider.id)
    thirdPartyProviders.value = thirdPartyProviders.value.map((item) => (
      item.provider === provider.id ? { ...item, bound: false } : item
    ))
    ElMessage.success(`${provider.name} 授权已撤销`)
  } catch (error) {
    if (error.status === 401) {
      router.push('/login?redirect=/profile')
      return
    }
    ElMessage.error(error.message || '撤销授权失败')
  } finally {
    unbindingProvider.value = ''
  }
}

const revokeDevice = async (device) => {
  if (device.current || revokingDevice.value) {
    return
  }

  try {
    await ElMessageBox.confirm(
      `踢出后，${device.name} 上的所有登录将立即失效，确定继续吗？`,
      '踢出登录设备',
      {
        confirmButtonText: '踢出设备',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
  } catch {
    return
  }

  revokingDevice.value = device.device_id
  try {
    await userAPI.revokeDevice(device.device_id)
    devices.value = devices.value.filter((item) => item.device_id !== device.device_id)
    ElMessage.success('设备已踢出')
  } catch (error) {
    if (error.status === 401) {
      router.push('/login?redirect=/profile')
      return
    }
    if (error.status === 404) {
      await loadDevices()
    }
    ElMessage.error(error.message || '踢出设备失败')
  } finally {
    revokingDevice.value = ''
  }
}

const describeUserAgent = (userAgent = '') => {
  let browser = ''
  if (/Edg\//.test(userAgent)) {
    browser = 'Edge'
  } else if (/Firefox\/|FxiOS\//.test(userAgent)) {
    browser = 'Firefox'
  } else if (/Chrome\/|CriOS\//.test(userAgent)) {
    browser = 'Chrome'
  } else if (/Safari\//.test(userAgent)) {
    browser = 'Safari'
  }

  let system = ''
  if (/Android/.test(userAgent)) {
    system = 'Android'
  } else if (/iPhone|iPad|iPod/.test(userAgent)) {
    system = 'iOS'
  } else if (/Windows/.test(userAgent)) {
    system = 'Windows'
  } else if (/Macintosh|Mac OS X/.test(userAgent)) {
    system = 'macOS'
  } else if (/Linux/.test(userAgent)) {
    system = 'Linux'
  }

  const name = [browser, system].filter(Boolean).join(' · ') || '未知设备'
  return {
    name,
    mobile: /Mobile|Android|iPhone|iPad|iPod/.test(userAgent)
  }
}

const authMethodLabel = (method) => ({
  PASSWORD: '密码登录',
  EMAIL_OTP: '邮箱验证码',
  QR: '扫码登录',
  GITHUB: 'GitHub 登录',
  FEISHU: '飞书登录'
})[method] || '其他登录方式'

const formatDate = (value) => {
  if (!value) {
    return '-'
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
  loadDevices()
})

onBeforeUnmount(() => {
  window.clearTimeout(copyTimer)
})
</script>

<style scoped>
.profile-page {
  min-height: 100vh;
  padding: 32px 24px;
  background:
    radial-gradient(circle at 20% 0%, rgba(8, 145, 178, 0.1), transparent 28%),
    linear-gradient(180deg, #f8fafc 0%, #eff6f5 100%);
  color: #172033;
}

.account-hero,
.profile-grid {
  width: min(1080px, 100%);
  margin: 0 auto;
}

.account-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 22px;
  padding: 18px 0;
}

.account-summary {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 16px;
}

.avatar {
  width: 72px;
  height: 72px;
  flex: 0 0 72px;
  overflow: hidden;
  border-radius: 20px;
  background: linear-gradient(135deg, #0891b2 0%, #0f766e 100%);
  box-shadow: 0 18px 36px rgba(8, 145, 178, 0.18);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  font-size: 30px;
  font-weight: 800;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.account-copy {
  min-width: 0;
}

.account-copy h1 {
  margin: 0;
  overflow: hidden;
  color: #0f172a;
  font-size: 30px;
  line-height: 1.18;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-copy p {
  margin: 7px 0 0;
  color: #64748b;
  font-size: 15px;
}

button {
  border: 0;
  cursor: pointer;
  font: inherit;
}

.account-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.logout-button,
.admin-button,
.bind-button,
.unbind-button,
.device-revoke-button,
.retry-button,
.text-button,
.icon-button {
  border-radius: 8px;
  font-weight: 700;
  transition: border-color 0.2s ease, color 0.2s ease, background 0.2s ease, transform 0.2s ease;
}

.logout-button {
  height: 42px;
  padding: 0 16px;
  border: 1px solid #cbd5e1;
  background: rgba(255, 255, 255, 0.86);
  color: #334155;
}

.admin-button {
  height: 42px;
  padding: 0 16px;
  background: #0f766e;
  color: #ffffff;
}

.logout-button:hover {
  border-color: #0891b2;
  color: #0e7490;
  transform: translateY(-1px);
}

.admin-button:hover {
  background: #115e59;
  transform: translateY(-1px);
}

.icon-button {
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #dbe5ee;
  background: #ffffff;
  color: #0f766e;
}

.icon-button:hover:not(:disabled) {
  border-color: #0891b2;
  background: #ecfeff;
  transform: translateY(-1px);
}

.icon-button:disabled,
.text-button:disabled {
  cursor: not-allowed;
  opacity: 0.48;
}

.profile-grid {
  display: grid;
  grid-template-columns: minmax(0, 0.95fr) minmax(300px, 0.75fr);
  gap: 16px;
}

.panel {
  min-width: 0;
  border: 1px solid rgba(203, 213, 225, 0.86);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.06);
  padding: 22px;
}

.devices-panel,
.apps-panel {
  grid-column: 1 / -1;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.panel-header h2 {
  margin: 0;
  color: #0f172a;
  font-size: 18px;
  line-height: 1.2;
}

.panel-header span,
.field-row span,
.provider-row span,
.app-head span,
.app-row time,
.app-row > span {
  color: #64748b;
  font-size: 14px;
}

.device-list {
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.device-row {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) auto;
  align-items: center;
  gap: 14px;
  min-width: 0;
  padding: 16px;
  border-top: 1px solid #e2e8f0;
}

.device-row:first-child {
  border-top: 0;
}

.device-icon {
  width: 42px;
  height: 42px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  background: #ecfeff;
  color: #0e7490;
}

.device-content {
  min-width: 0;
}

.device-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.device-title strong {
  color: #111827;
  font-weight: 750;
}

.current-device-badge {
  border-radius: 999px;
  background: #ccfbf1;
  color: #0f766e;
  padding: 3px 8px;
  font-size: 12px;
  font-weight: 800;
}

.device-agent {
  display: block;
  overflow: hidden;
  margin-top: 4px;
  color: #94a3b8;
  font-size: 12px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 16px;
  margin-top: 8px;
}

.device-meta span {
  color: #64748b;
  font-size: 13px;
}

.device-revoke-button,
.retry-button {
  height: 34px;
  padding: 0 12px;
  border: 1px solid #fca5a5;
  background: #ffffff;
  color: #dc2626;
  font-size: 14px;
}

.device-revoke-button:hover:not(:disabled),
.retry-button:hover:not(:disabled) {
  border-color: #ef4444;
  background: #fef2f2;
  transform: translateY(-1px);
}

.device-revoke-button:disabled {
  cursor: not-allowed;
  opacity: 0.48;
}

.device-error-state {
  justify-content: space-between;
  gap: 12px;
}

.field-list,
.provider-list {
  display: grid;
  gap: 0;
}

.field-row,
.provider-row {
  display: grid;
  align-items: center;
  min-width: 0;
  border-top: 1px solid #edf2f7;
  padding: 13px 0;
}

.field-row:first-child,
.provider-row:first-child {
  border-top: 0;
}

.field-row {
  grid-template-columns: 92px minmax(0, 1fr);
  gap: 16px;
}

.id-value,
.username-value {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 10px;
}

.id-value strong,
.username-value strong {
  min-width: 0;
  flex: 1 1 auto;
}

.dialog-mask {
  position: fixed;
  inset: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 18px;
  background: rgba(15, 23, 42, 0.45);
}

.dialog {
  display: grid;
  width: min(420px, 100%);
  max-height: calc(100vh - 36px);
  overflow: auto;
  gap: 16px;
  border-radius: 8px;
  background: #ffffff;
  padding: 20px;
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.25);
}

.dialog header,
.dialog footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dialog h2 {
  margin: 0;
  color: #0f172a;
  font-size: 18px;
  line-height: 1.2;
}

.dialog label {
  display: grid;
  gap: 8px;
  color: #334155;
  font-size: 14px;
  font-weight: 750;
}

.dialog input {
  width: 100%;
  box-sizing: border-box;
  min-width: 0;
  height: 40px;
  border: 1px solid #dbe5ee;
  border-radius: 8px;
  background: #ffffff;
  color: #111827;
  font: inherit;
  font-weight: 700;
  outline: none;
  padding: 0 12px;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.dialog input:focus {
  border-color: #0891b2;
  box-shadow: 0 0 0 3px rgba(8, 145, 178, 0.12);
}

.dialog footer {
  justify-content: flex-end;
}

.text-button {
  min-width: 72px;
  height: 38px;
  padding: 0 16px;
}

.text-button.primary {
  background: #0891b2;
  color: #ffffff;
}

.text-button.primary:hover:not(:disabled) {
  background: #0e7490;
  transform: translateY(-1px);
}

.text-button.secondary {
  border: 1px solid #cbd5e1;
  background: #ffffff;
  color: #334155;
}

.text-button.secondary:hover {
  border-color: #0891b2;
  color: #0e7490;
  transform: translateY(-1px);
}

.field-row strong,
.provider-row strong,
.app-row strong,
.app-link {
  min-width: 0;
  overflow: hidden;
  color: #111827;
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-link {
  text-decoration: none;
}

.app-link:hover {
  color: #0e7490;
  text-decoration: underline;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
}

.provider-row {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
}

.provider-row div {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.bind-button {
  height: 34px;
  min-width: 62px;
  padding: 0 14px;
  background: #0891b2;
  color: #ffffff;
  font-size: 14px;
}

.bind-button:hover:not(:disabled) {
  background: #0e7490;
  transform: translateY(-1px);
}

.unbind-button {
  height: 34px;
  min-width: 84px;
  padding: 0 12px;
  border: 1px solid #fca5a5;
  background: #ffffff;
  color: #dc2626;
  font-size: 14px;
}

.unbind-button:hover:not(:disabled) {
  border-color: #ef4444;
  background: #fef2f2;
  transform: translateY(-1px);
}

.bind-button:disabled,
.unbind-button:disabled {
  cursor: not-allowed;
  opacity: 0.48;
}

.app-table {
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.app-row {
  display: grid;
  grid-template-columns: minmax(170px, 1fr) minmax(170px, 0.85fr) 120px;
  gap: 16px;
  align-items: center;
  min-width: 0;
  padding: 14px 16px;
  border-top: 1px solid #e2e8f0;
}

.app-row:first-child {
  border-top: 0;
}

.app-head {
  background: #f8fafc;
  font-weight: 700;
}

.app-row time {
  justify-self: end;
  color: #475569;
  font-weight: 700;
}

.empty-state {
  display: flex;
  align-items: center;
  min-height: 96px;
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  padding: 0 18px;
  color: #64748b;
  font-weight: 700;
}

@media (max-width: 820px) {
  .profile-page {
    padding: 24px 16px;
  }

  .account-hero {
    align-items: flex-start;
    flex-direction: column;
  }

  .profile-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 620px) {
  .account-summary {
    align-items: flex-start;
  }

  .avatar {
    width: 58px;
    height: 58px;
    flex-basis: 58px;
    border-radius: 16px;
    font-size: 24px;
  }

  .account-copy h1 {
    font-size: 24px;
  }

  .account-actions,
  .logout-button,
  .admin-button {
    width: 100%;
  }

  .panel {
    padding: 18px;
  }

  .field-row,
  .provider-row,
  .app-row {
    grid-template-columns: 1fr;
    gap: 6px;
  }

  .device-row {
    grid-template-columns: 42px minmax(0, 1fr);
    align-items: start;
  }

  .device-revoke-button {
    grid-column: 2;
    justify-self: start;
  }

  .device-error-state {
    align-items: flex-start;
    flex-direction: column;
  }

  .id-value,
  .username-value {
    gap: 8px;
  }

  .app-head {
    display: none;
  }

  .app-row time {
    justify-self: start;
  }
}
</style>

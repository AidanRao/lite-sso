<template>
  <main class="admin-page">
    <header class="topbar">
      <button class="icon-button" type="button" title="返回资料页" @click="router.push('/profile')">
        <ArrowLeft :size="18" />
      </button>
      <div>
        <h1>系统管理</h1>
      </div>
      <button class="icon-text-button primary" type="button" @click="openCreateDialog">
        <Plus :size="17" />
        <span>新增平台</span>
      </button>
    </header>

    <section class="admin-shell">
      <aside class="sidebar">
        <button :class="{ active: activeTab === 'users' }" type="button" @click="activeTab = 'users'">
          <Users :size="18" />
          <span>用户</span>
        </button>
        <button :class="{ active: activeTab === 'clients' }" type="button" @click="activeTab = 'clients'">
          <PanelsTopLeft :size="18" />
          <span>平台</span>
        </button>
      </aside>

      <section class="content">
        <div class="content-toolbar">
          <div>
            <h2>{{ activeTab === 'users' ? '系统用户' : '接入平台' }}</h2>
            <span>{{ activeTab === 'users' ? `${users.length} 条记录` : `${clients.length} 条记录` }}</span>
          </div>
          <button class="icon-button" type="button" title="刷新" :disabled="loading" @click="loadData">
            <RefreshCw :class="{ spinning: loading }" :size="18" />
          </button>
        </div>

        <div v-if="activeTab === 'users'" class="data-table users-table">
          <div class="table-row table-head">
            <span>用户</span>
            <span>用户 ID</span>
            <span>状态</span>
            <span>创建时间</span>
          </div>
          <div
            v-for="item in users"
            :key="item.id"
            class="table-row user-row"
            role="button"
            tabindex="0"
            @click="openUserDetail(item)"
            @keydown.enter.prevent="openUserDetail(item)"
            @keydown.space.prevent="openUserDetail(item)"
          >
            <div class="user-cell">
              <span class="avatar">
                <img v-if="item.avatar_url" :src="item.avatar_url" :alt="item.username || item.email || item.id" />
                <span v-else>{{ avatarInitial(item) }}</span>
              </span>
              <div>
                <strong>{{ item.username || item.email || '未命名用户' }}</strong>
                <span>{{ item.email || '邮箱未设置' }}</span>
              </div>
            </div>
            <span class="mono">{{ item.id }}</span>
            <span>
              <span :class="['status', item.is_active ? 'ok' : 'disabled']">{{ item.is_active ? '启用' : '停用' }}</span>
              <span v-if="item.is_admin" class="status admin">管理员</span>
            </span>
            <time>{{ formatDate(item.created_at) }}</time>
          </div>
          <div v-if="!users.length && !loading" class="empty-state">暂无用户</div>
        </div>

        <div v-else class="data-table clients-table">
          <div class="table-row table-head">
            <span>平台</span>
            <span>Homepage URL</span>
            <span>回调地址</span>
            <span></span>
          </div>
          <div v-for="client in clients" :key="client.id" class="table-row">
            <strong>{{ client.name }}</strong>
            <span class="uri-list">{{ client.homepage_url }}</span>
            <span class="uri-list">{{ client.redirect_uri }}</span>
            <button class="icon-button" type="button" title="编辑平台" @click="openEditDialog(client)">
              <Pencil :size="17" />
            </button>
          </div>
          <div v-if="!clients.length && !loading" class="empty-state">暂无接入平台</div>
        </div>
      </section>
    </section>

    <div v-if="dialogOpen" class="dialog-mask" @click.self="closeDialog">
      <form class="dialog" @submit.prevent="saveClient">
        <header>
          <h2>{{ editingClient ? '编辑平台' : '新增平台' }}</h2>
          <button class="icon-button" type="button" title="关闭" @click="closeDialog">
            <X :size="18" />
          </button>
        </header>

        <label>
          <span>平台名称</span>
          <input v-model.trim="form.name" required maxlength="50" />
        </label>
        <label>
          <span>Client ID</span>
          <input v-model.trim="form.client_id" required maxlength="50" />
        </label>
        <label>
          <span>Homepage URL</span>
          <input v-model.trim="form.homepage_url" required maxlength="255" />
        </label>
        <div class="secret-field">
          <span>Client Secret</span>
          <div class="secret-row">
            <output class="secret-output mono">{{ secretLoading ? '密钥加载中' : (clientSecretDisplay || '未获取密钥') }}</output>
            <button
              class="icon-button"
              type="button"
              title="复制密钥"
              :disabled="!form.client_secret || secretLoading"
              @click="copyClientSecret"
            >
              <Copy :size="17" />
            </button>
            <button
              class="icon-button"
              type="button"
              :title="secretVisible ? '隐藏密钥' : '显示密钥'"
              :disabled="!form.client_secret || secretLoading"
              @click="secretVisible = !secretVisible"
            >
              <EyeOff v-if="secretVisible" :size="17" />
              <Eye v-else :size="17" />
            </button>
            <button class="icon-button" type="button" title="重新生成密钥" @click="regenerateClientSecret">
              <RefreshCw :size="17" />
            </button>
          </div>
        </div>
        <label>
          <span>回调地址</span>
          <input v-model.trim="form.redirect_uri" required maxlength="255" />
        </label>
        <label>
          <span>登出通知地址</span>
          <input v-model.trim="form.logout_uri" maxlength="255" />
        </label>

        <footer>
          <button class="icon-text-button secondary" type="button" @click="closeDialog">
            <X :size="17" />
            <span>取消</span>
          </button>
          <button class="icon-text-button primary" type="submit" :disabled="saving || secretLoading">
            <Save :size="17" />
            <span>保存</span>
          </button>
        </footer>
      </form>
    </div>

    <div v-if="userDetailOpen" class="dialog-mask" @click.self="closeUserDetail">
      <section class="detail-panel">
        <header>
          <div class="detail-identity">
            <span class="detail-avatar">
              <img
                v-if="selectedUserDetail?.user?.avatar_url"
                :src="selectedUserDetail.user.avatar_url"
                :alt="selectedUserDetail.user.username || selectedUserDetail.user.email || selectedUserDetail.user.id"
              />
              <span v-else>{{ avatarInitial(selectedUserDetail?.user || {}) }}</span>
            </span>
            <div>
              <h2>{{ selectedUserDetail?.user?.username || selectedUserDetail?.user?.email || '未命名用户' }}</h2>
              <p>{{ selectedUserDetail?.user?.email || '邮箱未设置' }}</p>
            </div>
          </div>
          <button class="icon-button" type="button" title="关闭" @click="closeUserDetail">
            <X :size="18" />
          </button>
        </header>

        <div v-if="detailLoading" class="detail-loading">加载中</div>
        <template v-else>
          <section class="detail-section">
            <h3>账号</h3>
            <div class="detail-fields">
              <div>
                <span>用户 ID</span>
                <strong class="mono">{{ selectedUserDetail?.user?.id || '-' }}</strong>
              </div>
              <div>
                <span>状态</span>
                <strong>{{ selectedUserDetail?.user?.is_active ? '启用' : '停用' }}</strong>
              </div>
              <div>
                <span>管理员</span>
                <strong>{{ selectedUserDetail?.user?.is_admin ? '是' : '否' }}</strong>
              </div>
              <div>
                <span>创建时间</span>
                <strong>{{ formatDate(selectedUserDetail?.user?.created_at) }}</strong>
              </div>
            </div>
          </section>

          <section class="detail-section">
            <h3>登录应用</h3>
            <div v-if="selectedUserDetail?.applications?.length" class="detail-list">
              <div v-for="app in selectedUserDetail.applications" :key="app.client_id" class="detail-list-row">
                <div>
                  <strong>{{ app.name || app.client_id }}</strong>
                  <span class="mono">{{ app.client_id }}</span>
                </div>
                <time>{{ formatDate(app.last_login_at) }}</time>
              </div>
            </div>
            <div v-else class="empty-state compact">暂无登录应用</div>
          </section>

          <section class="detail-section">
            <h3>第三方登录</h3>
            <div class="provider-chips">
              <span
                v-for="provider in selectedUserDetail?.third_party_providers || []"
                :key="provider.provider"
                :class="['status', provider.bound ? 'ok' : 'disabled']"
              >
                {{ providerName(provider.provider) }} · {{ provider.bound ? '已绑定' : '未绑定' }}
              </span>
            </div>
          </section>
        </template>
      </section>
    </div>
  </main>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Copy, Eye, EyeOff, PanelsTopLeft, Pencil, Plus, RefreshCw, Save, Users, X } from 'lucide-vue-next'
import { adminAPI } from '../api/auth'
import { generateClientSecret, maskClientSecret } from '../utils/clientSecret'

const router = useRouter()
const activeTab = ref('users')
const loading = ref(false)
const saving = ref(false)
const dialogOpen = ref(false)
const userDetailOpen = ref(false)
const detailLoading = ref(false)
const editingClient = ref(null)
const selectedUserDetail = ref(null)
const secretVisible = ref(false)
const secretLoading = ref(false)
const users = ref([])
const clients = ref([])
const form = reactive({
  name: '',
  client_id: '',
  client_secret: '',
  homepage_url: '',
  redirect_uri: '',
  logout_uri: ''
})
const clientSecretDisplay = computed(() => {
  if (secretVisible.value) {
    return form.client_secret
  }
  return maskClientSecret(form.client_secret)
})

const loadData = async () => {
  loading.value = true
  try {
    const [userResponse, clientResponse] = await Promise.all([
      adminAPI.listUsers(),
      adminAPI.listOAuthClients()
    ])
    users.value = userResponse?.data?.users || []
    clients.value = clientResponse?.data?.clients || []
  } catch (error) {
    if (error.status === 401) {
      router.push('/login?redirect=/admin')
      return
    }
    if (error.status === 403) {
      ElMessage.error('无管理员权限')
      router.push('/profile')
      return
    }
    ElMessage.error(error.message || '加载管理数据失败')
  } finally {
    loading.value = false
  }
}

const openCreateDialog = () => {
  editingClient.value = null
  resetForm()
  regenerateClientSecret()
  dialogOpen.value = true
}

const openUserDetail = async (user) => {
  userDetailOpen.value = true
  detailLoading.value = true
  selectedUserDetail.value = { user, applications: [], third_party_providers: [] }
  try {
    const response = await adminAPI.getUserDetail(user.id)
    selectedUserDetail.value = response?.data?.profile || selectedUserDetail.value
  } catch (error) {
    ElMessage.error(error.message || '获取用户详情失败')
    closeUserDetail()
  } finally {
    detailLoading.value = false
  }
}

const closeUserDetail = () => {
  userDetailOpen.value = false
  detailLoading.value = false
  selectedUserDetail.value = null
}

const openEditDialog = async (client) => {
  editingClient.value = client
  form.name = client.name || ''
  form.client_id = client.client_id || ''
  form.client_secret = ''
  form.homepage_url = client.homepage_url || ''
  secretVisible.value = false
  form.redirect_uri = client.redirect_uri || ''
  form.logout_uri = client.logout_uri || ''
  dialogOpen.value = true
  await loadClientSecret(client.id)
}

const closeDialog = () => {
  dialogOpen.value = false
  editingClient.value = null
  resetForm()
}

const saveClient = async () => {
  saving.value = true
  try {
    if (!editingClient.value && !form.client_secret) {
      regenerateClientSecret()
    }
    const payload = {
      name: form.name,
      client_id: form.client_id,
      homepage_url: form.homepage_url,
      redirect_uri: form.redirect_uri,
      logout_uri: form.logout_uri
    }
    if (!editingClient.value || form.client_secret.trim()) {
      payload.client_secret = form.client_secret.trim()
    }

    if (editingClient.value) {
      await adminAPI.updateOAuthClient(editingClient.value.id, payload)
      ElMessage.success('平台已更新')
    } else {
      await adminAPI.createOAuthClient(payload)
      ElMessage.success('平台已新增')
    }
    closeDialog()
    await loadData()
  } catch (error) {
    ElMessage.error(error.message || '保存平台失败')
  } finally {
    saving.value = false
  }
}

const resetForm = () => {
  form.name = ''
  form.client_id = ''
  form.client_secret = ''
  form.homepage_url = ''
  secretVisible.value = false
  secretLoading.value = false
  form.redirect_uri = ''
  form.logout_uri = ''
}

const regenerateClientSecret = () => {
  form.client_secret = generateClientSecret()
  secretVisible.value = false
}

const loadClientSecret = async (id) => {
  secretLoading.value = true
  try {
    const response = await adminAPI.getOAuthClientSecret(id)
    form.client_secret = response?.data?.secret?.client_secret || ''
    secretVisible.value = false
  } catch (error) {
    ElMessage.error(error.message || '获取平台密钥失败')
  } finally {
    secretLoading.value = false
  }
}

const copyClientSecret = async () => {
  if (!form.client_secret) {
    return
  }
  try {
    await navigator.clipboard.writeText(form.client_secret)
    ElMessage.success('密钥已复制')
  } catch (error) {
    ElMessage.error(error.message || '复制密钥失败')
  }
}

const avatarInitial = (item) => {
  return (item.username || item.email || item.id || '?').slice(0, 1).toUpperCase()
}

const providerName = (provider) => {
  const names = {
    github: 'GitHub',
    feishu: '飞书'
  }
  return names[provider] || provider
}

const formatDate = (value) => {
  if (!value) {
    return '-'
  }
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value))
}

onMounted(loadData)
</script>

<style scoped>
.admin-page {
  min-height: 100vh;
  padding: 28px 24px 40px;
  background: #f6f8fb;
  color: #111827;
}

.topbar,
.admin-shell {
  width: min(1180px, 100%);
  margin: 0 auto;
}

.topbar {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr) auto;
  gap: 14px;
  align-items: center;
  margin-bottom: 18px;
}

.topbar h1,
.content-toolbar h2,
.dialog h2 {
  margin: 0;
  color: #0f172a;
  line-height: 1.2;
}

.topbar h1 {
  font-size: 28px;
}

.topbar p,
.content-toolbar span,
.table-head span,
.user-cell span,
.table-row time {
  color: #64748b;
  font-size: 14px;
}

.topbar p {
  margin: 6px 0 0;
}

button,
input,
textarea {
  font: inherit;
}

button {
  border: 0;
  cursor: pointer;
}

button:disabled {
  cursor: not-allowed;
  opacity: 0.62;
}

.icon-button,
.icon-text-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  transition: background 0.2s ease, border-color 0.2s ease, color 0.2s ease, transform 0.2s ease;
}

.icon-button {
  width: 40px;
  height: 40px;
  border: 1px solid #dbe4ee;
  background: #ffffff;
  color: #334155;
}

.icon-button:hover {
  border-color: #94a3b8;
  transform: translateY(-1px);
}

.icon-text-button {
  height: 40px;
  gap: 8px;
  padding: 0 14px;
  font-weight: 750;
}

.primary {
  background: #0f766e;
  color: #ffffff;
}

.primary:hover {
  background: #115e59;
  transform: translateY(-1px);
}

.secondary {
  border: 1px solid #cbd5e1;
  background: #ffffff;
  color: #334155;
}

.secondary:hover {
  border-color: #94a3b8;
}

.admin-shell {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.sidebar,
.content {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 16px 36px rgba(15, 23, 42, 0.05);
}

.sidebar {
  display: grid;
  gap: 4px;
  padding: 8px;
}

.sidebar button {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 42px;
  border-radius: 8px;
  background: transparent;
  color: #475569;
  padding: 0 12px;
  font-weight: 750;
  text-align: left;
}

.sidebar button.active,
.sidebar button:hover {
  background: #e6fffb;
  color: #0f766e;
}

.content {
  min-width: 0;
  padding: 18px;
}

.content-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 14px;
}

.content-toolbar h2 {
  font-size: 19px;
}

.data-table {
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.table-row {
  display: grid;
  gap: 14px;
  align-items: center;
  min-width: 0;
  padding: 13px 14px;
  border-top: 1px solid #e2e8f0;
}

.table-row:first-child {
  border-top: 0;
}

.user-row {
  cursor: pointer;
}

.user-row:hover,
.user-row:focus-visible {
  background: #f8fafc;
}

.user-row:focus-visible {
  outline: 2px solid rgba(15, 118, 110, 0.26);
  outline-offset: -2px;
}

.table-head {
  background: #f8fafc;
  font-weight: 750;
}

.users-table .table-row {
  grid-template-columns: minmax(190px, 1.1fr) minmax(180px, 1fr) minmax(150px, 0.7fr) 150px;
}

.clients-table .table-row {
  grid-template-columns: minmax(160px, 0.9fr) minmax(150px, 0.7fr) minmax(240px, 1.2fr) 46px;
}

.table-row strong,
.table-row > span,
.table-row time {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-cell {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 11px;
}

.user-cell > div {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  border-radius: 8px;
  background: #dff7f3;
  color: #0f766e;
  font-weight: 850;
  overflow: hidden;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
}

.status {
  display: inline-flex;
  align-items: center;
  height: 26px;
  margin-right: 6px;
  border-radius: 999px;
  padding: 0 9px;
  font-size: 12px;
  font-weight: 800;
}

.status.ok {
  background: #dcfce7;
  color: #166534;
}

.status.disabled {
  background: #fee2e2;
  color: #991b1b;
}

.status.admin {
  background: #e0f2fe;
  color: #075985;
}

.uri-list {
  white-space: pre-line;
}

.empty-state {
  display: flex;
  align-items: center;
  min-height: 96px;
  padding: 0 16px;
  border-top: 1px solid #e2e8f0;
  color: #64748b;
  font-weight: 750;
}

.empty-state.compact {
  min-height: 58px;
  border-top: 0;
}

.dialog-mask {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 18px;
  background: rgba(15, 23, 42, 0.45);
  z-index: 20;
}

.dialog {
  display: grid;
  gap: 14px;
  width: min(560px, 100%);
  max-height: calc(100vh - 36px);
  overflow: auto;
  border-radius: 8px;
  background: #ffffff;
  padding: 20px;
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.25);
}

.detail-panel {
  display: grid;
  gap: 18px;
  width: min(720px, 100%);
  max-height: calc(100vh - 36px);
  overflow: auto;
  border-radius: 8px;
  background: #ffffff;
  padding: 20px;
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.25);
}

.dialog header,
.dialog footer,
.detail-panel header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dialog footer {
  justify-content: flex-end;
  padding-top: 4px;
}

.dialog label,
.secret-field {
  display: grid;
  gap: 7px;
  color: #334155;
  font-size: 14px;
  font-weight: 750;
}

.secret-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) repeat(3, 40px);
  gap: 8px;
  align-items: center;
}

.secret-output {
  display: flex;
  align-items: center;
  min-width: 0;
  height: 40px;
  box-sizing: border-box;
  overflow: hidden;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #f8fafc;
  color: #0f172a;
  padding: 0 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dialog input,
.dialog textarea {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #ffffff;
  color: #0f172a;
  padding: 11px 12px;
  outline: none;
  resize: vertical;
}

.dialog input:focus,
.dialog textarea:focus {
  border-color: #0f766e;
  box-shadow: 0 0 0 3px rgba(15, 118, 110, 0.14);
}

.detail-identity {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 13px;
}

.detail-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 54px;
  height: 54px;
  flex: 0 0 54px;
  overflow: hidden;
  border-radius: 8px;
  background: #dff7f3;
  color: #0f766e;
  font-size: 20px;
  font-weight: 850;
}

.detail-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.detail-identity h2,
.detail-section h3 {
  margin: 0;
  color: #0f172a;
  line-height: 1.2;
}

.detail-identity h2 {
  overflow: hidden;
  font-size: 20px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-identity p {
  margin: 5px 0 0;
  color: #64748b;
  font-size: 14px;
}

.detail-loading {
  display: flex;
  align-items: center;
  min-height: 160px;
  color: #64748b;
  font-weight: 750;
}

.detail-section {
  display: grid;
  gap: 12px;
}

.detail-section h3 {
  font-size: 16px;
}

.detail-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.detail-fields > div {
  display: grid;
  min-width: 0;
  gap: 5px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 12px;
  background: #f8fafc;
}

.detail-fields span,
.detail-list-row span,
.detail-list-row time {
  color: #64748b;
  font-size: 13px;
}

.detail-fields strong,
.detail-list-row strong {
  min-width: 0;
  overflow: hidden;
  color: #111827;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-list {
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}

.detail-list-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 160px;
  gap: 12px;
  align-items: center;
  padding: 12px 14px;
  border-top: 1px solid #e2e8f0;
}

.detail-list-row:first-child {
  border-top: 0;
}

.detail-list-row > div {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.detail-list-row time {
  justify-self: end;
  color: #475569;
  font-weight: 750;
}

.provider-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.spinning {
  animation: spin 0.9s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 900px) {
  .admin-shell {
    grid-template-columns: 1fr;
  }

  .sidebar {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .users-table .table-row,
  .clients-table .table-row {
    grid-template-columns: 1fr;
  }

  .table-head {
    display: none;
  }

  .clients-table .table-row .icon-button {
    width: 100%;
  }
}

@media (max-width: 620px) {
  .admin-page {
    padding: 20px 14px 32px;
  }

  .topbar {
    grid-template-columns: 40px minmax(0, 1fr);
  }

  .topbar .primary {
    grid-column: 1 / -1;
    width: 100%;
  }

  .content {
    padding: 14px;
  }

  .dialog footer {
    display: grid;
    grid-template-columns: 1fr;
  }

  .secret-row {
    grid-template-columns: minmax(0, 1fr) repeat(3, 40px);
  }

  .detail-fields,
  .detail-list-row {
    grid-template-columns: 1fr;
  }

  .detail-list-row time {
    justify-self: start;
  }
}
</style>

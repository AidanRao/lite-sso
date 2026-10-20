<template>
  <section aria-labelledby="users-title">
    <header class="page-heading">
      <div><h1 id="users-title">用户管理</h1></div>
      <button class="admin-button" type="button" :disabled="loading" @click="loadUsers"><RefreshCw :size="15" :class="{ spinning: loading }" />刷新</button>
    </header>
    <p v-if="loading" class="page-state" role="status">正在加载用户…</p>
    <div v-else-if="loadError" class="admin-error" role="alert">{{ loadError }} <button type="button" @click="loadUsers">重试</button></div>
    <template v-else>
      <p class="list-count">{{ users.length }} 位用户</p>
      <div class="data-table users-table">
        <div class="table-row table-head">
          <span>用户</span>
          <span>用户 ID</span>
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
          <time>{{ formatDate(item.created_at) }}</time>
        </div>
        <div v-if="!users.length && !loading" class="empty-state">暂无用户</div>
      </div>
    </template>
    <div v-if="userDetailOpen" class="dialog-mask" @click.self="closeUserDetail">
      <section class="detail-panel" role="dialog" aria-modal="true" aria-label="用户详情" @keydown="handleDetailKeydown">
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
          <button ref="detailCloseButton" class="icon-button" type="button" title="关闭" @click="closeUserDetail">
            <X :size="18" />
          </button>
        </header>

        <div v-if="detailLoading" class="detail-loading">加载中</div>
        <p v-else-if="detailError" class="admin-error" role="alert">{{ detailError }}</p>
        <template v-else>
          <section class="detail-section">
            <h3>账号</h3>
            <div class="detail-fields">
              <div>
                <span>用户 ID</span>
                <strong class="mono">{{ selectedUserDetail?.user?.id || '-' }}</strong>
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
                <div class="detail-app-identity">
                  <ApplicationLogo :label="app.name || app.client_id" :src="app.logo_url" size="small" />
                  <div>
                    <strong>{{ app.name || app.client_id }}</strong>
                    <span class="mono">{{ app.client_id }}</span>
                  </div>
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
  </section>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { RefreshCw, X } from 'lucide-vue-next'
import { adminAPI } from '../../api/auth'
import ApplicationLogo from '../../components/ApplicationLogo.vue'
import { useAdminAccess } from './useAdminAccess'

const { handleAccessError } = useAdminAccess()
const loading = ref(true)
const loadError = ref('')
const users = ref([])
const userDetailOpen = ref(false)
const detailLoading = ref(false)
const detailError = ref('')
const selectedUserDetail = ref(null)
const detailCloseButton = ref(null)
let detailVersion = 0
let detailTrigger = null

const loadUsers = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const response = await adminAPI.listUsers()
    users.value = response?.data?.users || []
  } catch (error) {
    if (!handleAccessError(error)) loadError.value = error.message || '加载用户失败'
  } finally {
    loading.value = false
  }
}

const openUserDetail = async (user) => {
  const version = ++detailVersion
  detailTrigger = document.activeElement
  userDetailOpen.value = true
  detailLoading.value = true
  detailError.value = ''
  selectedUserDetail.value = { user, applications: [], third_party_providers: [] }
  await nextTick()
  detailCloseButton.value?.focus()
  try {
    const response = await adminAPI.getUserDetail(user.id)
    if (version === detailVersion) selectedUserDetail.value = response?.data?.profile || selectedUserDetail.value
  } catch (error) {
    if (version === detailVersion && !handleAccessError(error)) detailError.value = error.message || '获取用户详情失败'
  } finally {
    if (version === detailVersion) detailLoading.value = false
  }
}

const closeUserDetail = () => {
  detailVersion++
  userDetailOpen.value = false
  selectedUserDetail.value = null
  nextTick(() => detailTrigger?.focus())
}

const handleDetailKeydown = event => {
  if (event.key === 'Escape') {
    event.preventDefault()
    closeUserDetail()
  } else if (event.key === 'Tab') {
    event.preventDefault()
    detailCloseButton.value?.focus()
  }
}

const avatarInitial = (item) => {
  return (item.username || item.email || item.id || '?').slice(0, 1).toUpperCase()
}

const providerName = (provider) => {
  const names = {
    github: 'GitHub',
    feishu: '飞书',
    qq: 'QQ',
    google: 'Google'
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

onMounted(loadUsers)
onBeforeUnmount(() => { detailVersion++ })
</script>

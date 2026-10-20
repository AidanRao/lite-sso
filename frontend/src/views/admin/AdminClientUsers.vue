<template>
  <section aria-labelledby="client-users-title">
    <header class="page-heading">
      <div><h2 id="client-users-title">Users</h2></div>
      <button class="admin-button danger-text" type="button" :disabled="loading || busy || !total" @click="askRemove(null)">清空全部用户</button>
    </header>
    <p class="field-description users-removal-note">移除只撤销此客户端的关联与授权，保留用户的 SSO 账号。用户以后可以重新登录。</p>
    <p v-if="loading" class="page-state" role="status">正在加载用户…</p>
    <div v-else-if="loadError" class="admin-error" role="alert">{{ loadError }} <button type="button" @click="loadUsers">重试</button></div>
    <template v-else>
      <div v-if="!users.length" class="empty-clients"><Users :size="28" /><h2>没有关联用户</h2></div>
      <div v-else class="client-users-table">
        <div class="client-user-row client-user-head"><span>用户</span><span>用户 ID</span><span>最后登录</span><span></span></div>
        <div v-for="user in users" :key="user.user_id" class="client-user-row">
          <div class="client-user-identity"><strong>{{ user.username || user.email || '未命名用户' }}</strong><span>{{ user.email || '邮箱未设置' }}</span></div>
          <span class="mono">{{ user.user_id }}</span><time>{{ formatDate(user.last_login_at) }}</time>
          <button class="admin-button danger-text" type="button" :disabled="busy" :aria-label="`移除 ${user.username || user.user_id}`" @click="askRemove(user)">移除</button>
        </div>
      </div>
      <nav class="client-pagination" aria-label="客户端用户分页">
        <span>共 {{ total }} 位用户 · 第 {{ page }} / {{ totalPages }} 页</span>
        <div><button class="admin-button" type="button" :disabled="page <= 1 || busy" @click="goPage(page - 1)">上一页</button><button class="admin-button" type="button" :disabled="page >= totalPages || busy" @click="goPage(page + 1)">下一页</button></div>
      </nav>
    </template>
    <ClientConfirmDialog :open="!!removal" :busy="busy" :title="removal?.all ? '清空全部用户？' : '移除此用户？'" :confirm-text="removal?.all ? '清空全部用户' : '移除用户'" :error="confirmError" @close="removal = null" @confirm="removeUsers">
      <p v-if="removal?.all">将撤销此客户端所有 {{ total }} 位用户的关联与授权，包括其他分页中的用户。</p>
      <p v-else>将撤销 {{ removal?.user?.username || removal?.user?.email || removal?.user?.user_id }} 在此客户端的关联与授权。</p>
      <p>用户的 SSO 账号和其他客户端授权会保留。</p>
    </ClientConfirmDialog>
  </section>
</template>

<script setup>
import { computed, inject, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Users } from 'lucide-vue-next'
import { adminAPI } from '../../api/auth'
import { CLIENT_CONTEXT_KEY } from './clientContext'
import { useAdminAccess } from './useAdminAccess'
import ClientConfirmDialog from './ClientConfirmDialog.vue'

const { client, update } = inject(CLIENT_CONTEXT_KEY)
const { handleAccessError } = useAdminAccess()
const route = useRoute()
const router = useRouter()
const integer = (value, fallback, maximum) => Number.isInteger(Number(value)) && Number(value) >= 1 && Number(value) <= maximum ? Number(value) : fallback
const page = computed(() => integer(route.query.page, 1, 1000000))
const pageSize = computed(() => integer(route.query.page_size, 20, 100))
const total = ref(client.value.user_count)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const users = ref([])
const loading = ref(true)
const loadError = ref('')
const busy = ref(false)
const removal = ref(null)
const confirmError = ref('')
let version = 0
let disposed = false
const goPage = value => router.replace({ query: { ...route.query, page: String(value) } })
const loadUsers = async () => {
  const current = ++version
  loading.value = true
  loadError.value = ''
  try {
    const response = await adminAPI.listOAuthClientUsers(client.value.id, { page: page.value, page_size: pageSize.value })
    if (current !== version || disposed) return
    total.value = response.data.total
    update({ user_count: total.value })
    if (page.value > totalPages.value) { await goPage(totalPages.value); return }
    users.value = response.data.users
  } catch (error) {
    if (current === version && !disposed && !handleAccessError(error)) loadError.value = error.message || '加载用户失败'
  } finally { if (current === version) loading.value = false }
}
const askRemove = user => { confirmError.value = ''; removal.value = user ? { user } : { all: true } }
const removeUsers = async () => {
  if (busy.value || !removal.value) return
  busy.value = true
  confirmError.value = ''
  const all = removal.value.all
  try {
    if (all) await adminAPI.clearOAuthClientUsers(client.value.id)
    else await adminAPI.removeOAuthClientUser(client.value.id, removal.value.user.user_id)
    if (disposed) return
    removal.value = null
    if (all && page.value !== 1) await goPage(1)
    else await loadUsers()
    ElMessage.success({ message: all ? '全部用户已移除' : '用户已移除', customClass: 'admin-toast' })
  } catch (error) {
    if (!disposed && !handleAccessError(error)) confirmError.value = error.message || '移除用户失败'
  } finally { busy.value = false }
}
const formatDate = value => value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value)) : '-'
watch([page, pageSize], loadUsers, { immediate: true })
onBeforeUnmount(() => { disposed = true; version++ })
</script>

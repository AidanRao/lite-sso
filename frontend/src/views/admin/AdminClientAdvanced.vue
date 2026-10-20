<template>
  <section aria-labelledby="advanced-title">
    <header class="page-heading"><div><h2 id="advanced-title">Advanced</h2></div></header>
    <h2 class="danger-zone-title">危险操作</h2>
    <div class="danger-zone">
      <div><h3>删除客户端</h3><p>彻底删除客户端配置，删除后无法恢复。</p><p v-if="client.user_count">仍有 {{ client.user_count }} 位关联用户，请先在 <RouterLink :to="`/admin/clients/${client.id}/users`">Users</RouterLink> 中移除全部用户。</p><p v-else>当前没有关联用户，可以删除客户端。</p></div>
      <button class="admin-button danger-text" type="button" :disabled="checking || !!pageError || client.user_count > 0 || busy" @click="openDelete">删除客户端</button>
    </div>
    <p v-if="pageError" class="admin-error" role="alert">{{ pageError }} <button type="button" @click="checkCount">重试</button></p>
    <ClientConfirmDialog :open="deleteOpen" :busy="busy" title="删除此客户端？" confirm-text="彻底删除客户端" :error="deleteError" @close="deleteOpen = false" @confirm="deleteClient">
      <p>将彻底删除 <strong>{{ client.name }}</strong>（{{ client.client_id }}）。此操作无法恢复。</p>
      <p>删除后无法继续登录或刷新此客户端的令牌；业务端离线验证的 JWT 仍按自身有效期过期。</p>
    </ClientConfirmDialog>
  </section>
</template>

<script setup>
import { inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { adminAPI } from '../../api/auth'
import { CLIENT_CONTEXT_KEY } from './clientContext'
import { useAdminAccess } from './useAdminAccess'
import ClientConfirmDialog from './ClientConfirmDialog.vue'

const { client, update, reload } = inject(CLIENT_CONTEXT_KEY)
const { handleAccessError } = useAdminAccess()
const router = useRouter()
const checking = ref(true)
const busy = ref(false)
const deleteOpen = ref(false)
const deleteError = ref('')
const pageError = ref('')
let disposed = false
const checkCount = async () => {
  checking.value = true
  pageError.value = ''
  try { await reload() }
  catch (error) { if (!disposed) pageError.value = error.message || '检查关联用户失败' }
  finally { checking.value = false }
}
const openDelete = () => { deleteError.value = ''; deleteOpen.value = true }
const deleteClient = async () => {
  if (busy.value) return
  busy.value = true
  deleteError.value = ''
  try {
    await adminAPI.deleteOAuthClient(client.value.id)
    if (disposed) return
    deleteOpen.value = false
    ElMessage.success({ message: '客户端已删除', customClass: 'admin-toast' })
    await router.push('/admin/clients')
  } catch (error) {
    if (disposed || handleAccessError(error)) return
    if (error.data?.code === 'CLIENT_HAS_USERS') {
      update({ user_count: error.data.user_count })
      deleteOpen.value = false
      pageError.value = '客户端有新的关联用户，请先前往 Users 移除。'
    } else deleteError.value = error.message || '删除客户端失败'
  } finally { busy.value = false }
}
onMounted(checkCount)
onBeforeUnmount(() => { disposed = true })
</script>

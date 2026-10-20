<template>
  <div>
    <div class="client-overview">
      <div class="client-id-block"><h2>Client ID</h2><code>{{ client.client_id }}</code></div>
    </div>
    <section v-if="client.client_type === 'confidential'" class="form-section client-secret-card" aria-labelledby="detail-secret-title">
      <header class="form-section-heading"><h2 id="detail-secret-title">Client Secret</h2><button class="admin-button" type="button" :disabled="loadingSecret" @click="rotateOpen = true">重新生成密钥</button></header>
      <p class="field-description">仅由应用后端保管。重新生成后，旧密钥立即失效。</p>
      <div class="secret-row detail-secret-row">
        <output class="secret-output mono" aria-label="Client Secret">{{ secret || 'xxx' }}</output>
        <button class="admin-button" type="button" :disabled="loadingSecret" @click="toggleSecret">{{ loadingSecret ? '正在加载…' : secret ? '隐藏' : '显示' }}</button>
        <button class="admin-button" type="button" :disabled="!secret || loadingSecret" @click="copySecret">复制</button>
      </div>
      <p v-if="secretError" class="admin-error" role="alert">{{ secretError }}</p>
    </section>
    <AdminClientForm detail :client="client" @saved="updated" />
    <ClientConfirmDialog :open="rotateOpen" :busy="rotating" title="重新生成 Client Secret？" confirm-text="生成新密钥" :error="rotateError" @close="rotateOpen = false" @confirm="rotateSecret"><p>旧密钥将立即失效，请及时更新应用配置。</p></ClientConfirmDialog>
  </div>
</template>

<script setup>
import { inject, onBeforeUnmount, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adminAPI } from '../../api/auth'
import { CLIENT_CONTEXT_KEY } from './clientContext'
import { useAdminAccess } from './useAdminAccess'
import AdminClientForm from './AdminClientForm.vue'
import ClientConfirmDialog from './ClientConfirmDialog.vue'

const { client, update } = inject(CLIENT_CONTEXT_KEY)
const { handleAccessError } = useAdminAccess()
const secret = ref('')
const secretError = ref('')
const loadingSecret = ref(false)
const rotateOpen = ref(false)
const rotating = ref(false)
const rotateError = ref('')
let requestVersion = 0
const updated = value => { requestVersion++; secret.value = ''; loadingSecret.value = false; secretError.value = ''; rotateOpen.value = false; update(value) }
const toggleSecret = async () => {
  if (secret.value) { requestVersion++; secret.value = ''; return }
  const version = ++requestVersion
  loadingSecret.value = true
  secretError.value = ''
  try {
    const response = await adminAPI.getOAuthClientSecret(client.value.id)
    if (version === requestVersion) secret.value = response.data.secret.client_secret
  } catch (error) {
    if (version === requestVersion && !handleAccessError(error)) secretError.value = error.message || '读取密钥失败'
  } finally { if (version === requestVersion) loadingSecret.value = false }
}
const copySecret = async () => {
  try { await navigator.clipboard.writeText(secret.value); ElMessage.success({ message: '密钥已复制', customClass: 'admin-toast' }) }
  catch { secretError.value = '复制失败，请手动复制。' }
}
const rotateSecret = async () => {
  rotating.value = true
  rotateError.value = ''
  const version = ++requestVersion
  try {
    const response = await adminAPI.rotateOAuthClientSecret(client.value.id)
    if (version !== requestVersion) return
    secret.value = response.data.secret.client_secret
    rotateOpen.value = false
    ElMessage.success({ message: '新密钥已生成', customClass: 'admin-toast' })
  } catch (error) {
    if (version === requestVersion && !handleAccessError(error)) rotateError.value = error.message || '生成密钥失败'
  } finally { rotating.value = false }
}
onBeforeUnmount(() => { requestVersion++; secret.value = '' })
</script>

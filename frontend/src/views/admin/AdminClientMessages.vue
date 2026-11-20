<template>
  <section aria-labelledby="client-messages-title">
    <header class="page-heading">
      <h2 id="client-messages-title">发送消息</h2>
      <RouterLink class="admin-button" :to="usersPath">返回 Users</RouterLink>
    </header>
    <p class="field-description message-note">邮件发送至所选用户已验证的主邮箱，所有收件人共用下方参数。发送期间请保持页面打开；离开后停止后续提交，已提交的消息仍会投递。草稿和发送记录仅保留在当前页面。</p>
    <form class="client-form message-form" @submit.prevent="startSend">
      <fieldset :disabled="busy">
        <div class="form-columns">
          <div class="form-field"><label for="message-sender">sender-key <span class="required-mark">*</span></label><input id="message-sender" v-model="senderKey" required maxlength="100" autocomplete="off" placeholder="例如 sso" /></div>
          <div class="form-field"><label for="message-template">template-key <span class="required-mark">*</span></label><input id="message-template" v-model="templateKey" required maxlength="100" autocomplete="off" placeholder="填写已发布的模板 key" /></div>
        </div>
        <div class="form-field"><label for="message-variables">公共参数（JSON 对象） <span class="required-mark">*</span></label><textarea id="message-variables" v-model="variablesText" class="mono" rows="7" spellcheck="false" required /><small>例如 {"title":"通知","count":2}；所有用户共用，不自动添加个人信息。最多 64 KiB。</small></div>
      </fieldset>

      <section aria-labelledby="message-recipients-title">
        <header class="message-section-heading"><h3 id="message-recipients-title">选择收件人</h3><div class="message-selection-actions"><span>已选 {{ selected.size }} 人</span><button class="admin-button" type="button" :disabled="busy || !selected.size" @click="selected.clear()">清除选择</button></div></header>
        <p v-if="loading" class="page-state" role="status">正在加载用户…</p>
        <div v-else-if="loadError" class="admin-error" role="alert">{{ loadError }} <button type="button" @click="loadUsers">重试</button></div>
        <div v-else-if="!users.length" class="page-state">没有关联用户</div>
        <div v-else class="message-users-table">
          <label class="message-user-row message-user-head"><input type="checkbox" :checked="allPageSelected" :indeterminate="somePageSelected && !allPageSelected" :disabled="busy || !eligibleUsers.length" @change="selectPage($event.target.checked)" /><span>当前页全选</span><span>收件邮箱</span></label>
          <label v-for="user in users" :key="user.user_id" class="message-user-row" :class="{ 'message-user-unavailable': !user.email }">
            <input type="checkbox" :checked="selected.has(user.user_id)" :disabled="busy || !user.email" :aria-label="`选择 ${user.username || user.email || user.user_id}`" @change="selectUser(user, $event.target.checked)" />
            <span class="client-user-identity"><strong>{{ user.username || '未命名用户' }}</strong><span class="mono">{{ user.user_id }}</span></span>
            <span>{{ user.email || '无已验证的主邮箱，无法发送' }}</span>
          </label>
        </div>
        <nav class="client-pagination" aria-label="收件人分页"><span>共 {{ total }} 位用户 · 第 {{ page }} / {{ totalPages }} 页</span><div><button class="admin-button" type="button" :disabled="loading || busy || page <= 1" @click="page--">上一页</button><button class="admin-button" type="button" :disabled="loading || busy || page >= totalPages" @click="page++">下一页</button></div></nav>
      </section>
      <p v-if="formError" class="admin-error" role="alert">{{ formError }}</p>
      <div class="message-send-actions"><button class="admin-button primary" type="submit" :disabled="busy || !selected.size">{{ busy ? '正在提交…' : run ? `开始新一轮发送（${selected.size} 人）` : `发送给 ${selected.size} 位用户` }}</button><button v-if="run && retryCount" class="admin-button" type="button" :disabled="busy" @click="submitRun">重试未成功项（{{ retryCount }}）</button></div>
    </form>

    <section v-if="run" class="message-results" aria-labelledby="message-results-title">
      <header class="message-section-heading"><h3 id="message-results-title">本轮提交结果</h3><span role="status" aria-live="polite">已处理 {{ processedCount }} / {{ run.entries.length }} · 已提交 {{ acceptedCount }}</span></header>
      <p class="field-description">本轮 sender-key：{{ run.settings.sender_key }} · template-key：{{ run.settings.template_key }}。“已提交”表示消息中心已接受请求，不代表邮件已送达。重试使用本轮原始参数和幂等键，已提交的用户会被跳过。</p>
      <ul class="message-result-list"><li v-for="entry in run.entries" :key="entry.user_id"><div><strong>{{ entry.username || entry.email }}</strong><span class="mono">{{ entry.user_id }}</span></div><div><strong :class="`message-status-${entry.status}`">{{ statusLabels[entry.status] }}</strong><span v-if="entry.error">{{ entry.error }}</span></div></li></ul>
    </section>
  </section>
</template>

<script setup>
import { computed, inject, onBeforeUnmount, ref, watch } from 'vue'
import { adminAPI } from '../../api/auth'
import { createMessageRun, parseMessageSettings, sendMessageRun, setMessageSelection } from '../../utils/clientMessages'
import { CLIENT_CONTEXT_KEY } from './clientContext'
import { useAdminAccess } from './useAdminAccess'

const { client } = inject(CLIENT_CONTEXT_KEY)
const { handleAccessError } = useAdminAccess()
const usersPath = computed(() => `/admin/clients/${client.value.id}/users`)
const senderKey = ref('')
const templateKey = ref('')
const variablesText = ref('{}')
const selected = ref(new Map())
const users = ref([])
const page = ref(1)
const total = ref(0)
const pageSize = 20
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const loading = ref(true)
const loadError = ref('')
const formError = ref('')
const busy = ref(false)
const run = ref(null)
const statusLabels = { pending: '待提交', sending: '提交中…', accepted: '已提交', failed: '失败', unknown: '结果未知' }
const eligibleUsers = computed(() => users.value.filter(user => user.email?.trim()))
const allPageSelected = computed(() => eligibleUsers.value.length > 0 && eligibleUsers.value.every(user => selected.value.has(user.user_id)))
const somePageSelected = computed(() => eligibleUsers.value.some(user => selected.value.has(user.user_id)))
const acceptedCount = computed(() => run.value?.entries.filter(entry => entry.status === 'accepted').length || 0)
const processedCount = computed(() => run.value?.entries.filter(entry => ['accepted', 'failed', 'unknown'].includes(entry.status)).length || 0)
const retryCount = computed(() => run.value?.entries.filter(entry => entry.status !== 'accepted').length || 0)
let disposed = false
let loadVersion = 0

const selectUser = (user, checked) => setMessageSelection(selected.value, [user], checked)
const selectPage = checked => setMessageSelection(selected.value, users.value, checked)
const loadUsers = async () => {
  const version = ++loadVersion
  loading.value = true
  loadError.value = ''
  try {
    const response = await adminAPI.listOAuthClientUsers(client.value.id, { page: page.value, page_size: pageSize })
    if (disposed || version !== loadVersion) return
    total.value = response.data.total
    if (page.value > totalPages.value) { page.value = totalPages.value; return }
    users.value = response.data.users
  } catch (error) {
    if (!disposed && version === loadVersion && !handleAccessError(error)) loadError.value = error.message || '加载用户失败'
  } finally {
    if (!disposed && version === loadVersion) loading.value = false
  }
}

const submitRun = async () => {
  if (busy.value || !run.value) return
  busy.value = true
  await sendMessageRun(run.value, (entry, settings) => adminAPI.sendOAuthClientUserMessage(client.value.id, entry.user_id, settings, entry.key), {
    shouldStop: () => disposed,
    onAccessError: error => { if (!disposed) handleAccessError(error) }
  }).finally(() => { busy.value = false })
}
const startSend = async () => {
  if (busy.value) return
  formError.value = ''
  try {
    const settings = parseMessageSettings(senderKey.value, templateKey.value, variablesText.value)
    if (!selected.value.size) throw new Error('请至少选择一位收件人。')
    run.value = createMessageRun([...selected.value.values()], settings)
  } catch (error) {
    formError.value = error.message
    return
  }
  await submitRun()
}

watch(page, loadUsers, { immediate: true })
onBeforeUnmount(() => { disposed = true; loadVersion++ })
</script>

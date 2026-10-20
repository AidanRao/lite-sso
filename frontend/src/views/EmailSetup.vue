<template>
  <main class="setup-page">
    <section class="setup-card" aria-labelledby="setup-title" :aria-busy="loading || busy">
      <h1 id="setup-title">设置邮箱，让账号更安全</h1>
      <p class="description">邮箱可用于登录、找回密码和接收安全通知。即使第三方账号无法使用，也能多一种登录方式。</p>

      <p v-if="loading" class="status" role="status">正在检查邮箱…</p>
      <template v-else-if="loadFailed">
        <p class="error" role="alert">{{ error }}</p>
        <button class="button primary" type="button" @click="load">重新加载</button>
      </template>
      <template v-else>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <p v-if="notice" class="notice" role="status">{{ notice }}</p>

        <template v-if="verified">
          <p class="status"><CircleCheck :size="18" aria-hidden="true" />邮箱已验证，可以继续。</p>
          <button class="button primary" type="button" @click="continueToTarget">继续</button>
        </template>
        <template v-else>
          <section v-if="selectedEmail && !editing" class="pending-email" aria-label="待验证邮箱">
            <label v-if="pendingEmails.length > 1" for="pending-email">待验证邮箱</label>
            <select v-if="pendingEmails.length > 1" id="pending-email" v-model="selectedID" :disabled="Boolean(busy)">
              <option v-for="item in pendingEmails" :key="item.id" :value="item.id">{{ item.email }}</option>
            </select>
            <strong v-else>{{ selectedEmail.email }}</strong>
            <p>请打开验证邮件中的链接，验证后回到此页面继续。链接有效期为 30 分钟。</p>
            <div class="pending-actions">
              <button class="text-button" type="button" :disabled="Boolean(busy)" @click="resend">{{ busy === 'send' ? '发送中…' : '重新发送' }}</button>
              <button class="text-button" type="button" :disabled="Boolean(busy)" @click="startEditing">更换邮箱</button>
            </div>
          </section>

          <form v-else @submit.prevent="send">
            <label for="setup-email">邮箱地址</label>
            <input id="setup-email" v-model.trim="email" type="email" autocomplete="email" maxlength="100" placeholder="请输入常用邮箱" :disabled="Boolean(busy) || atLimit" required />
            <p v-if="atLimit" class="hint">已达到邮箱数量上限，请前往<RouterLink to="/profile/access/emails">邮箱管理</RouterLink>删除不再使用的地址。</p>
            <button class="button primary" type="submit" :disabled="Boolean(busy) || atLimit || !email">{{ busy === 'send' ? '发送中…' : '发送验证邮件' }}</button>
            <button v-if="selectedEmail" class="text-button cancel-edit" type="button" :disabled="Boolean(busy)" @click="editing = false">返回待验证邮箱</button>
          </form>

          <button v-if="selectedEmail && !editing" class="button primary" type="button" :disabled="Boolean(busy)" @click="checkVerified">{{ busy === 'check' ? '正在检查…' : '我已验证，继续' }}</button>
          <p class="hint">{{ selectedEmail && !editing ? '未收到邮件？请检查垃圾邮件，也可以重新发送或更换邮箱。' : '我们会向此地址发送验证链接，验证成功后即可用于登录。' }}</p>
        </template>
      </template>

      <button v-if="!verified" class="text-button skip" type="button" :disabled="busy || loading" @click="continueToTarget">暂时跳过</button>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CircleCheck } from 'lucide-vue-next'
import { userAPI } from '../api/auth'
import { getEmailSetupRedirect } from '../utils/emailSetup'
import { isReauthCancelled } from '../utils/reauthCoordinator'

const route = useRoute()
const router = useRouter()
const emails = ref([])
const maxAddresses = ref(3)
const selectedID = ref('')
const email = ref('')
const editing = ref(false)
const loading = ref(true)
const loadFailed = ref(false)
const busy = ref('')
const error = ref('')
const notice = ref('')
const pendingEmails = computed(() => emails.value.filter(item => !item.verified))
const verified = computed(() => emails.value.some(item => item.verified))
const selectedEmail = computed(() => pendingEmails.value.find(item => item.id === selectedID.value))
const atLimit = computed(() => emails.value.length >= maxAddresses.value)
const target = computed(() => getEmailSetupRedirect(route.query.redirect))

const continueToTarget = () => window.location.assign(target.value)
const login = () => router.replace({ path: '/login', query: { redirect: route.fullPath } })

const applyEmails = (records) => {
  emails.value = records
  if (!pendingEmails.value.some(item => item.id === selectedID.value)) selectedID.value = pendingEmails.value[0]?.id || ''
}

const fetchEmails = async () => {
  const result = await userAPI.getEmails()
  applyEmails(Array.isArray(result?.data?.emails) ? result.data.emails : [])
  maxAddresses.value = Number(result?.data?.max_addresses) || 3
}

const load = async () => {
  loading.value = true
  loadFailed.value = false
  error.value = ''
  try {
    await fetchEmails()
  } catch (cause) {
    if (cause.status === 401) { await login(); return }
    loadFailed.value = true
    error.value = cause.message || '无法加载邮箱，请重试。'
  } finally { loading.value = false }
}

const handleError = async (cause) => {
  if (isReauthCancelled(cause)) return
  if (cause.status === 401) { await login(); return }
  if (cause.data?.email) {
    const record = cause.data.email
    applyEmails([...emails.value.filter(item => item.id !== record.id), record])
    selectedID.value = record.id
    editing.value = false
  }
  if (cause.data?.code === 'EMAIL_ALREADY_ADDED') {
    try {
      await fetchEmails()
      selectedID.value = pendingEmails.value.find(item => item.email.toLowerCase() === email.value.toLowerCase())?.id || selectedID.value
      editing.value = false
    } catch (refreshError) {
      await handleError(refreshError)
      return
    }
  }
  error.value = cause.message || '邮箱操作失败，请重试。'
}

const rememberSent = () => {
  editing.value = false
  notice.value = '验证邮件已发送，请查看收件箱。'
}

const send = async () => {
  if (busy.value || !email.value || atLimit.value) return
  busy.value = 'send'
  error.value = ''
  notice.value = ''
  try {
    const existing = pendingEmails.value.find(item => item.email.toLowerCase() === email.value.toLowerCase())
    if (existing) {
      selectedID.value = existing.id
      editing.value = false
      await userAPI.resendEmailVerification(existing.id)
    } else {
      const result = await userAPI.addEmail(email.value)
      const record = result.data.email
      applyEmails([...emails.value, record])
      selectedID.value = record.id
    }
    rememberSent()
  } catch (cause) { await handleError(cause) }
  finally { busy.value = '' }
}

const resend = async () => {
  if (busy.value || !selectedEmail.value) return
  busy.value = 'send'
  error.value = ''
  notice.value = ''
  try {
    await userAPI.resendEmailVerification(selectedEmail.value.id)
    rememberSent()
  } catch (cause) { await handleError(cause) }
  finally { busy.value = '' }
}

const checkVerified = async () => {
  if (busy.value) return
  busy.value = 'check'
  error.value = ''
  notice.value = ''
  try {
    await fetchEmails()
    if (verified.value) continueToTarget()
    else error.value = '邮箱尚未验证，请先点击验证邮件中的链接。'
  } catch (cause) { await handleError(cause) }
  finally { busy.value = '' }
}

const startEditing = () => {
  email.value = ''
  editing.value = true
  error.value = ''
  notice.value = ''
}

onMounted(load)
</script>

<style scoped>
.setup-page { min-height: 100dvh; box-sizing: border-box; display: flex; align-items: flex-start; justify-content: center; padding: 72px 20px; background: var(--profile-surface); color: var(--profile-text-strong); }
.setup-card { width: min(440px, 100%); box-sizing: border-box; padding: 28px; border: 1px solid var(--profile-border); border-radius: 10px; background: var(--profile-surface); }
h1 { margin: 0 0 12px; font-size: 23px; line-height: 1.4; }
p { font-size: 14px; line-height: 1.7; }
.description { margin: 0 0 24px; color: var(--profile-text-muted); }
label { display: block; margin-bottom: 8px; font-size: 14px; font-weight: 600; }
input, select { width: 100%; min-height: 42px; box-sizing: border-box; padding: 8px 11px; border: 1px solid var(--profile-border); border-radius: 6px; background: var(--profile-surface); color: inherit; font: inherit; font-size: 14px; }
input:focus, select:focus { outline: 2px solid var(--profile-focus-ring); border-color: var(--profile-accent); }
.button { display: inline-flex; justify-content: center; align-items: center; width: 100%; min-height: 40px; box-sizing: border-box; padding: 8px 12px; border: 1px solid var(--profile-accent); border-radius: 6px; font: inherit; font-size: 14px; font-weight: 600; cursor: pointer; }
.primary { margin-top: 16px; background: var(--profile-accent); color: #fff; }
.primary:hover:not(:disabled) { background: var(--profile-accent-hover); }
.text-button { padding: 0; border: 0; background: transparent; color: var(--profile-accent); font: inherit; font-size: 14px; cursor: pointer; }
.text-button:hover:not(:disabled) { text-decoration: underline; }
.skip, .cancel-edit { display: block; margin: 20px auto 0; }
.skip { color: var(--profile-text-muted); }
.hint { margin: 12px 0 0; color: var(--profile-text-muted); font-size: 12px; }
.hint a { color: var(--profile-accent); }
.pending-email { padding: 16px; border: 1px solid var(--profile-border); border-radius: 6px; background: var(--profile-surface-subtle); }
.pending-email strong { overflow-wrap: anywhere; font-size: 15px; }
.pending-email p { margin: 8px 0 12px; color: var(--profile-text-muted); font-size: 13px; }
.pending-actions { display: flex; gap: 20px; }
.error, .notice { padding: 10px 12px; margin: 0 0 16px; border-radius: 6px; overflow-wrap: anywhere; }
.error { background: var(--profile-danger-soft); color: var(--profile-danger); }
.notice { background: var(--profile-success-soft); color: var(--profile-success); }
.status { display: flex; align-items: center; gap: 8px; }
button:disabled { opacity: .55; cursor: not-allowed; }
@media (max-width: 520px) { .setup-page { padding: 28px 12px; } .setup-card { padding: 22px 18px; } h1 { font-size: 21px; } }
</style>

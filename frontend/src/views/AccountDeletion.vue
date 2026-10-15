<template>
  <main class="deletion-page">
    <section class="status-card" aria-labelledby="status-title">
      <CircleAlert :size="30" class="status-icon" aria-hidden="true" />
      <h1 id="status-title">{{ restored ? '已撤销账号注销' : '账号注销' }}</h1>
      <p v-if="loading" role="status">正在获取注销状态…</p>
      <template v-else-if="restored">
        <p>账号已恢复。请重新登录，旧设备会话和旧授权不会恢复。</p>
        <RouterLink class="button primary" to="/login">重新登录</RouterLink>
      </template>
      <template v-else>
        <p v-if="summary">{{ deadlinePassed ? '冷静期已结束，正在确认注销处理状态，无法再撤销。' : '账号已进入 24 小时注销冷静期。' }}</p>
        <dl v-if="summary"><dt>冷静期截止时间（北京时间）</dt><dd>{{ formatDeletionTime(summary.deletion_scheduled_at) }}</dd></dl>
        <template v-if="authenticated">
          <p v-if="!deadlinePassed">冷静期内只能查看注销状态、撤销注销或退出。重新登录不会自动撤销注销。</p>
          <div class="actions">
            <button v-if="status?.can_cancel && !deadlinePassed" class="button primary" type="button" :disabled="submitting" @click="cancel">{{ submitting ? '正在处理…' : '撤销注销' }}</button>
            <button class="button" type="button" :disabled="submitting" @click="logout">退出登录</button>
          </div>
        </template>
        <template v-else>
          <p v-if="!deadlinePassed">{{ summary ? '当前已退出所有设备。冷静期内可重新登录查看状态或撤销注销。' : '请登录查看注销状态或撤销尚在冷静期内的注销申请。' }}</p>
          <RouterLink class="button primary" :to="deadlinePassed ? '/login' : { path: '/login', query: { redirect: '/account/deletion' } }">{{ deadlinePassed ? '返回登录' : '登录管理注销' }}</RouterLink>
        </template>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <button v-if="loadFailed" class="retry" type="button" @click="load">重新获取状态</button>
        <p class="footnote">最终注销后，本系统将释放邮箱、用户名和第三方身份。接入应用中的业务数据由各应用管理。</p>
      </template>
    </section>
  </main>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { CircleAlert } from 'lucide-vue-next'
import { authAPI, userAPI, clearAccessToken } from '../api/auth'
import { isReauthCancelled } from '../utils/reauthCoordinator'
import { formatDeletionTime, readDeletionSummary, rememberDeletion, forgetDeletion } from '../utils/accountDeletion'

const router = useRouter()
const status = ref(null)
const summary = ref(readDeletionSummary())
const loading = ref(true)
const authenticated = ref(false)
const restored = ref(false)
const submitting = ref(false)
const error = ref('')
const loadFailed = ref(false)
const now = ref(Date.now())
const deadlinePassed = computed(() => summary.value && now.value >= Date.parse(summary.value.deletion_scheduled_at))
let disposed = false
let timer
let fetching = false
let nextStatusCheck = 0
const load = async ({ silent = false } = {}) => {
  if (fetching || disposed) return
  fetching = true
  if (!silent) loading.value = true
  loadFailed.value = false
  error.value = ''
  try {
    const response = await userAPI.getAccountDeletion()
    if (disposed) return
    if (response.data.status === 'active') { forgetDeletion(); router.replace('/profile/account'); return }
    status.value = response.data
    summary.value = response.data
    rememberDeletion(response.data)
    authenticated.value = true
  } catch (cause) {
    if (disposed) return
    authenticated.value = false
    if (cause.status === 401 && deadlinePassed.value) {
      clearAccessToken()
      forgetDeletion()
      summary.value = null
      status.value = null
      router.replace('/login')
      return
    }
    if (cause.status !== 401) { error.value = cause.message || '无法获取注销状态'; loadFailed.value = true }
  } finally {
    fetching = false
    nextStatusCheck = Date.now() + 15000
    if (!disposed) loading.value = false
  }
}
const cancel = async () => {
  if (submitting.value || !status.value?.can_cancel || deadlinePassed.value) return
  submitting.value = true
  error.value = ''
  try {
    await userAPI.cancelAccountDeletion()
    clearAccessToken()
    forgetDeletion()
    authenticated.value = false
    restored.value = true
  } catch (cause) {
    if (!isReauthCancelled(cause)) error.value = cause.message || '撤销失败，请重试'
    if (cause.data?.code === 'ACCOUNT_DELETION_DUE') { status.value.can_cancel = false; now.value = Date.now() }
  } finally { submitting.value = false }
}
const logout = async () => {
  if (submitting.value) return
  submitting.value = true
  error.value = ''
  try {
    await authAPI.logout()
    clearAccessToken()
    authenticated.value = false
  } catch (cause) { error.value = cause.message || '退出失败，请重试' }
  finally { submitting.value = false }
}
onMounted(() => {
  void load()
  timer = window.setInterval(() => {
    now.value = Date.now()
    if (!restored.value && deadlinePassed.value && now.value >= nextStatusCheck) void load({ silent: true })
  }, 5000)
})
onBeforeUnmount(() => { disposed = true; window.clearInterval(timer) })
</script>

<style scoped>
.deletion-page { display: flex; justify-content: center; align-items: flex-start; min-height: 100dvh; box-sizing: border-box; padding: 64px 20px; background: var(--profile-surface); color: var(--profile-text-strong); }
.status-card { width: min(520px, 100%); box-sizing: border-box; border: 1px solid var(--profile-border); border-radius: 10px; padding: 28px; background: var(--profile-surface-subtle); }
.status-icon { color: var(--profile-danger); }
h1 { font-size: 24px; margin: 14px 0 20px; }
p { font-size: 14px; line-height: 1.7; }
dl { padding: 16px; border: 1px solid var(--profile-border); border-radius: 6px; margin: 20px 0; }
dt { color: var(--profile-text-muted); font-size: 12px; }
dd { font-size: 17px; margin: 8px 0 0; font-variant-numeric: tabular-nums; }
.actions { display: flex; gap: 12px; flex-wrap: wrap; margin-top: 22px; }
.button { display: inline-flex; justify-content: center; align-items: center; box-sizing: border-box; min-height: 36px; border: 1px solid var(--profile-border-muted); border-radius: 6px; padding: 7px 14px; background: var(--profile-surface); color: var(--profile-text-strong); font: inherit; font-size: 14px; font-weight: 600; text-decoration: none; cursor: pointer; }
.button.primary { border-color: var(--profile-accent); background: var(--profile-accent); color: #fff; }
button:disabled { opacity: .5; cursor: not-allowed; }
.error { color: var(--profile-danger); }
.footnote { margin-top: 24px; color: var(--profile-text-muted); font-size: 12px; }
.retry { color: var(--profile-accent); background: transparent; border: 0; cursor: pointer; }
@media (max-width: 560px) { .deletion-page { padding: 24px 12px; } .status-card { padding: 20px; } }
</style>

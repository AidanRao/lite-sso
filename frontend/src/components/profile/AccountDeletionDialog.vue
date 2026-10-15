<template>
  <Teleport to="body">
    <div class="deletion-mask" @click.self="close" @keydown="onKeydown">
      <section ref="dialog" class="deletion-dialog" role="dialog" aria-modal="true" aria-labelledby="deletion-title" aria-describedby="deletion-description" tabindex="-1">
        <header>
          <h2 id="deletion-title">确定要注销账号吗？</h2>
          <button class="close-button" type="button" aria-label="关闭" :disabled="submitting" @click="close"><X :size="20" /></button>
        </header>
        <div class="warning"><TriangleAlert :size="22" aria-hidden="true" /><strong>请仔细阅读，注销完成后无法恢复。</strong></div>
        <div id="deletion-description" class="description">
          <p>提交后将立即退出所有设备，并进入 <strong>24 小时冷静期</strong>。</p>
          <p>冷静期内，重新登录仅可查看注销状态或撤销注销。<strong>重新登录不会自动撤销注销。</strong></p>
          <p>冷静期结束后，将删除本系统的账号资料及登录凭证，释放邮箱、用户名和第三方身份。再次注册会创建全新账号。</p>
          <p>接入应用中的业务数据由各应用管理；已签发的应用访问令牌可能在有效期内继续生效。</p>
        </div>
        <p v-if="loading" role="status">正在获取账号信息…</p>
        <div v-else-if="loadError" class="error" role="alert">{{ loadError }} <button type="button" class="retry-button" @click="loadEmails">重试</button></div>
        <form v-else @submit.prevent="submit">
          <label for="deletion-identity">{{ usesID ? '请输入用户 ID' : '请输入当前用户名或已验证邮箱' }}</label>
          <p v-if="usesID" class="hint mono">{{ user.id }}</p>
          <input id="deletion-identity" ref="identityInput" v-model="identity" autocomplete="off" :disabled="submitting" />
          <label for="deletion-phrase">请输入「{{ deletionPhrase }}」以确认</label>
          <input id="deletion-phrase" v-model="phrase" autocomplete="off" :disabled="submitting" />
          <p v-if="error" class="error" role="alert">{{ error }}</p>
          <p class="hint">继续后，系统会确认你的身份，再提交注销申请。</p>
          <button class="delete-button" type="submit" :disabled="submitting || !matches">{{ submitting ? '正在处理…' : '继续注销账号' }}</button>
        </form>
      </section>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { TriangleAlert, X } from 'lucide-vue-next'
import { userAPI } from '../../api/auth'
import { isReauthCancelled } from '../../utils/reauthCoordinator'
import { deletionIdentifiers, deletionConfirmationMatches, deletionPhrase } from '../../utils/accountDeletion'

const props = defineProps({ user: { type: Object, required: true } })
const emit = defineEmits(['close', 'submitted'])
const dialog = ref(null)
const identityInput = ref(null)
const identity = ref('')
const phrase = ref('')
const emails = ref([])
const loading = ref(true)
const loadError = ref('')
const error = ref('')
const submitting = ref(false)
const identifiers = computed(() => deletionIdentifiers(props.user, emails.value))
const usesID = computed(() => !props.user.username && !emails.value.some(email => email.verified))
const matches = computed(() => deletionConfirmationMatches(identity.value, phrase.value, identifiers.value))
let disposed = false
let oldOverflow = ''
let returnFocus = null

const close = () => { if (!submitting.value) emit('close') }
const loadEmails = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const result = await userAPI.getEmails()
    if (disposed) return
    emails.value = result?.data?.emails || []
  } catch (cause) {
    if (!disposed) loadError.value = cause.message || '无法获取已验证邮箱'
  } finally {
    if (!disposed) { loading.value = false; await nextTick(); identityInput.value?.focus() }
  }
}
const submit = async () => {
  if (submitting.value || !matches.value || loading.value || loadError.value) return
  submitting.value = true
  error.value = ''
  try {
    const result = await userAPI.requestAccountDeletion()
    emit('submitted', result.data)
  } catch (cause) {
    if (!isReauthCancelled(cause)) error.value = cause.message || '提交失败，请重试'
  } finally {
    submitting.value = false
    await nextTick()
    if (!disposed) dialog.value?.focus()
  }
}
const onKeydown = (event) => {
  if (event.key === 'Escape') { event.preventDefault(); close(); return }
  if (event.key !== 'Tab') return
  const elements = [...dialog.value.querySelectorAll('button:not(:disabled), input:not(:disabled), a[href]')]
  if (!elements.length) { event.preventDefault(); return }
  const first = elements[0]
  const last = elements.at(-1)
  if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog.value)) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
onMounted(() => {
  returnFocus = document.activeElement
  oldOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  dialog.value?.focus()
  void loadEmails()
})
onBeforeUnmount(() => {
  disposed = true
  identity.value = ''
  phrase.value = ''
  document.body.style.overflow = oldOverflow
  if (returnFocus?.isConnected) returnFocus.focus()
})
</script>

<style scoped>
.deletion-mask { position: fixed; z-index: 150; inset: 0; display: flex; align-items: center; justify-content: center; padding: 20px; background: var(--profile-overlay); color: var(--profile-text-strong); }
.deletion-dialog { width: min(560px, 100%); max-height: calc(100dvh - 40px); overflow-y: auto; box-sizing: border-box; border: 1px solid var(--profile-border); border-radius: 12px; background: var(--profile-surface); box-shadow: var(--profile-shadow); padding: 24px; }
header { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 20px; }
h2 { margin: 0; font-size: 20px; }
.close-button { display: inline-flex; align-items: center; justify-content: center; border: 0; padding: 5px; border-radius: 5px; background: transparent; color: var(--profile-text-muted); cursor: pointer; }
.warning { display: flex; align-items: center; gap: 12px; padding: 16px; border-block: 1px solid var(--profile-danger); color: var(--profile-danger); background: color-mix(in srgb, var(--profile-danger) 8%, transparent); font-size: 14px; }
.warning svg { flex-shrink: 0; }
.description { font-size: 14px; line-height: 1.7; padding: 6px 0 14px; border-bottom: 1px solid var(--profile-border); }
.description p { margin: 12px 0 0; }
form { display: grid; gap: 9px; margin-top: 20px; }
label { font-size: 14px; font-weight: 600; }
label:not(:first-child) { margin-top: 8px; }
input { box-sizing: border-box; width: 100%; min-height: 38px; border: 1px solid var(--profile-border-muted); border-radius: 6px; padding: 7px 10px; background: var(--profile-surface-subtle); color: var(--profile-text-strong); font: inherit; }
input:focus { outline: 2px solid var(--profile-accent); outline-offset: 1px; }
.hint { margin: 5px 0; color: var(--profile-text-muted); font-size: 12px; line-height: 1.6; overflow-wrap: anywhere; }
.mono { font-family: ui-monospace, monospace; }
.delete-button { min-height: 40px; border: 1px solid var(--profile-danger); border-radius: 6px; background: var(--profile-danger); color: white; font: inherit; font-size: 14px; font-weight: 600; cursor: pointer; }
button:disabled { cursor: not-allowed; opacity: .45; }
.error { color: var(--profile-danger); font-size: 13px; }
.retry-button { border: 0; background: transparent; color: inherit; text-decoration: underline; cursor: pointer; }
@media (max-width: 560px) { .deletion-mask { padding: 12px; } .deletion-dialog { max-height: calc(100dvh - 24px); padding: 18px; } h2 { font-size: 18px; } }
</style>

<template>
  <div class="emails-view">
    <ProfileSettingsSection
      title-id="emails-title"
      title="Emails"
      description="已验证邮箱均可用于登录和找回密码。账号通知、二次验证和其他系统邮件默认发送到主邮箱。"
      divided
    >
      <div v-if="verificationState.message" class="verification-banner" :class="verificationState.type" role="status">
        <CircleCheck v-if="verificationState.type === 'success'" :size="19" aria-hidden="true" />
        <CircleAlert v-else :size="19" aria-hidden="true" />
        <span>{{ verificationState.message }}</span>
      </div>

      <div v-if="loading" class="page-state"><span class="spinner" aria-hidden="true"></span>正在加载邮箱…</div>
      <div v-else-if="loadError" class="page-state error-state">
        <span>{{ loadError }}</span>
        <button class="button" type="button" @click="loadEmails">重新加载</button>
      </div>
      <template v-else>
        <section class="email-list" aria-label="已绑定邮箱">
          <article v-for="item in emails" :key="item.id" class="email-row">
            <div class="email-content">
              <div class="email-heading">
                <strong>{{ item.email }}</strong>
                <span v-if="item.is_primary" class="tag primary-tag">Primary</span>
                <span v-if="item.verified" class="tag verified-tag">Verified</span>
                <span v-else class="tag unverified-tag">Unverified</span>
                <span v-for="source in item.sources" :key="source" class="tag source-tag">
                  <ThirdPartyProviderIcon :provider="source" :size="14" />
                  Connected to {{ providerName(source) }}
                </span>
              </div>
              <p v-if="item.is_primary" class="email-note">此邮箱用于账号通知、二次验证和其他默认邮件。</p>
              <button
                v-if="!item.verified"
                class="resend-link"
                type="button"
                :disabled="resendingID === item.id"
                @click="resendVerification(item)"
              >
                {{ resendingID === item.id ? '发送中…' : '重新发送验证邮件' }}
              </button>
            </div>

            <details class="more-menu">
              <summary class="icon-button" aria-label="更多操作"><Ellipsis :size="20" /></summary>
              <div class="menu-popover">
                <button class="menu-danger" type="button" :disabled="deletingID === item.id" @click="deleteEmail(item, $event)">
                  <Trash2 :size="16" />
                  {{ deletingID === item.id ? '删除中…' : '删除邮箱' }}
                </button>
              </div>
            </details>
          </article>
        </section>

        <form class="add-form" @submit.prevent="addEmail">
          <label for="new-email">新增邮箱地址 <span aria-hidden="true">*</span></label>
          <div class="add-controls">
            <input
              id="new-email"
              v-model="newEmail"
              type="email"
              autocomplete="email"
              placeholder="Email address"
              :disabled="adding || atLimit"
              required
            />
            <button class="button" type="submit" :disabled="adding || atLimit || !newEmail.trim()">
              {{ adding ? '添加中…' : 'Add' }}
            </button>
          </div>
          <p class="form-hint">{{ atLimit ? `当前账号最多可绑定 ${maxAddresses} 个邮箱。` : `已绑定 ${emails.length}/${maxAddresses} 个邮箱。新增后需在 30 分钟内完成验证。` }}</p>
        </form>

        <section class="primary-card" aria-labelledby="primary-email-title">
          <div>
            <h2 id="primary-email-title">Primary email address</h2>
            <p>选择账号通知、二次验证和默认系统邮件使用的邮箱。</p>
          </div>
          <select v-model="selectedPrimaryID" :disabled="changingPrimary || verifiedEmails.length < 2" @change="changePrimary">
            <option v-if="verifiedEmails.length === 0" value="">尚未设置主邮箱</option>
            <option v-for="item in verifiedEmails" :key="item.id" :value="item.id">{{ item.email }}</option>
          </select>
        </section>
      </template>
    </ProfileSettingsSection>
  </div>
</template>

<script setup>
import { computed, inject, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CircleAlert, CircleCheck, Ellipsis, Trash2 } from 'lucide-vue-next'

import { userAPI } from '../../api/auth'
import ProfileSettingsSection from '../../components/profile/ProfileSettingsSection.vue'
import ThirdPartyProviderIcon from '../../components/ThirdPartyProviderIcon.vue'
import { getEmailVerificationToken } from '../../utils/emailVerification'
import { isReauthCancelled } from '../../utils/reauthCoordinator'
import { PROFILE_CONTEXT_KEY } from './profileContext'

const route = useRoute()
const router = useRouter()
const profile = inject(PROFILE_CONTEXT_KEY)
if (!profile) throw new Error('ProfileEmails must be rendered inside Profile')

const emails = ref([])
const maxAddresses = ref(3)
const loading = ref(true)
const loadError = ref('')
const newEmail = ref('')
const adding = ref(false)
const resendingID = ref('')
const deletingID = ref('')
const changingPrimary = ref(false)
const selectedPrimaryID = ref('')
const previousPrimaryID = ref('')
const verificationState = reactive({ type: '', message: '' })

const atLimit = computed(() => emails.value.length >= maxAddresses.value)
const verifiedEmails = computed(() => emails.value.filter(item => item.verified))

const providerName = (provider) => ({ github: 'GitHub', feishu: '飞书' }[provider] || provider)

const syncPrimarySelection = () => {
  const current = emails.value.find(item => item.is_primary)
  selectedPrimaryID.value = current?.id || ''
  previousPrimaryID.value = selectedPrimaryID.value
}

const loadEmails = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const result = await userAPI.getEmails()
    const data = result?.data || {}
    emails.value = Array.isArray(data.emails) ? data.emails : []
    maxAddresses.value = Number(data.max_addresses) || 3
    syncPrimarySelection()
  } catch (error) {
    loadError.value = error.message || '邮箱加载失败'
  } finally {
    loading.value = false
  }
}

const addEmail = async () => {
  if (adding.value || atLimit.value || !newEmail.value.trim()) return
  adding.value = true
  try {
    await userAPI.addEmail(newEmail.value.trim())
    newEmail.value = ''
    ElMessage.success('验证邮件已发送')
    await loadEmails()
  } catch (error) {
    if (isReauthCancelled(error)) return
    if (error.data?.email) await loadEmails()
    ElMessage.error(error.message || '新增邮箱失败')
  } finally {
    adding.value = false
  }
}

const resendVerification = async (item) => {
  if (resendingID.value) return
  resendingID.value = item.id
  try {
    await userAPI.resendEmailVerification(item.id)
    ElMessage.success(`验证邮件已发送至 ${item.email}`)
  } catch (error) {
    ElMessage.error(error.message || '验证邮件发送失败')
  } finally {
    resendingID.value = ''
  }
}

const deleteEmail = async (item, event) => {
  event?.currentTarget?.closest('details')?.removeAttribute('open')
  if (deletingID.value) return
  try {
    await ElMessageBox.confirm(`确定删除邮箱 ${item.email}？删除后将无法再使用该邮箱登录。`, '删除邮箱', {
      confirmButtonText: '删除邮箱',
      cancelButtonText: '取消',
      type: 'warning',
      confirmButtonClass: 'danger-confirm-button'
    })
  } catch {
    return
  }
  deletingID.value = item.id
  try {
    await userAPI.deleteEmail(item.id)
    ElMessage.success('邮箱已删除')
    await loadEmails()
  } catch (error) {
    if (!isReauthCancelled(error)) ElMessage.error(error.message || '删除邮箱失败')
  } finally {
    deletingID.value = ''
  }
}

const changePrimary = async () => {
  if (!selectedPrimaryID.value || selectedPrimaryID.value === previousPrimaryID.value) return
  const nextID = selectedPrimaryID.value
  changingPrimary.value = true
  try {
    await userAPI.setPrimaryEmail(nextID)
    previousPrimaryID.value = nextID
    ElMessage.success('主邮箱已更新')
    await Promise.all([loadEmails(), profile.reload()])
  } catch (error) {
    selectedPrimaryID.value = previousPrimaryID.value
    if (!isReauthCancelled(error)) ElMessage.error(error.message || '主邮箱更新失败')
  } finally {
    changingPrimary.value = false
  }
}

const confirmFromLink = async () => {
  const token = getEmailVerificationToken(route.hash)
  if (!token) return
  await router.replace('/profile/access/emails')
  try {
    await userAPI.confirmEmailVerification(token)
    verificationState.type = 'success'
    verificationState.message = '邮箱验证成功，现在可以用于登录。'
    await Promise.all([loadEmails(), profile.reload()])
  } catch (error) {
    verificationState.type = 'error'
    verificationState.message = error.message || '邮箱验证失败，请重新发送验证邮件。'
  }
}

onMounted(async () => {
  await loadEmails()
  await confirmFromLink()
})
</script>

<style scoped>
.emails-view { color: #1f2328; }
.verification-banner { display: flex; align-items: center; gap: 9px; margin: 0; padding: 12px 14px; border: 1px solid; border-radius: 6px; font-size: 14px; }
.verification-banner.success { border-color: #1a7f37; background: #dafbe1; color: #116329; }
.verification-banner.error { border-color: #cf222e; background: #ffebe9; color: #a40e26; }
.page-state { display: flex; min-height: 180px; align-items: center; justify-content: center; gap: 10px; border: 1px solid #d0d7de; border-radius: 6px; color: #57606a; }
.page-state.error-state { flex-direction: column; color: #cf222e; }
.spinner { width: 18px; height: 18px; border: 2px solid #d0d7de; border-top-color: #0969da; border-radius: 50%; animation: spin .8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.email-list { overflow: visible; border: 1px solid #d0d7de; border-radius: 6px; }
.email-row { position: relative; display: flex; min-height: 92px; align-items: flex-start; justify-content: space-between; gap: 20px; padding: 20px 24px; }
.email-row + .email-row { border-top: 1px solid #d8dee4; }
.email-content { min-width: 0; }
.email-heading { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.email-heading strong { overflow-wrap: anywhere; font-size: 17px; font-weight: 600; }
.tag { display: inline-flex; min-height: 24px; box-sizing: border-box; align-items: center; gap: 5px; padding: 1px 8px; border: 1px solid #d0d7de; border-radius: 999px; background: #fff; font-size: 12px; font-weight: 600; }
.primary-tag { border-color: #54aeff; color: #0969da; }
.verified-tag { border-color: #2da44e; color: #1a7f37; }
.unverified-tag { border-color: #bf8700; color: #9a6700; }
.source-tag { color: #57606a; }
.email-note { margin: 8px 0 0; color: #57606a; font-size: 13px; line-height: 1.5; }
.resend-link { margin-top: 10px; padding: 0; border: 0; background: transparent; color: #0969da; font: inherit; font-size: 14px; cursor: pointer; }
.resend-link:hover:not(:disabled) { text-decoration: underline; }
.resend-link:disabled { color: #8c959f; cursor: default; }
.more-menu { position: relative; flex: 0 0 auto; }
.more-menu summary { list-style: none; }
.more-menu summary::-webkit-details-marker { display: none; }
.icon-button { display: inline-flex; width: 34px; height: 34px; align-items: center; justify-content: center; border: 0; border-radius: 6px; background: transparent; color: #57606a; cursor: pointer; }
.icon-button:hover { background: #f3f4f6; }
.menu-popover { position: absolute; z-index: 10; top: 38px; right: 0; width: 180px; padding: 6px; border: 1px solid #d0d7de; border-radius: 6px; background: #fff; box-shadow: 0 8px 24px rgba(140, 149, 159, .2); }
.menu-danger { display: flex; width: 100%; min-height: 34px; align-items: center; gap: 8px; padding: 7px 10px; border: 0; border-radius: 4px; background: transparent; color: #cf222e; font: inherit; font-size: 13px; font-weight: 600; cursor: pointer; }
.menu-danger:hover:not(:disabled) { background: #ffebe9; }
.menu-danger:disabled { color: #8c959f; cursor: not-allowed; }
.add-form { margin: 28px 0 30px; }
.add-form label { display: block; margin-bottom: 9px; font-size: 15px; font-weight: 600; }
.add-controls { display: flex; max-width: 620px; gap: 9px; }
.add-controls input { min-width: 0; min-height: 38px; flex: 1 1 auto; box-sizing: border-box; padding: 7px 12px; border: 1px solid #d0d7de; border-radius: 6px; background: #f6f8fa; color: #24292f; font: inherit; }
.add-controls input:focus { border-color: #0969da; outline: 2px solid rgba(9, 105, 218, .2); background: #fff; }
.button { min-height: 36px; padding: 6px 14px; border: 1px solid rgba(27, 31, 36, .15); border-radius: 6px; background: #f6f8fa; color: #24292f; font: inherit; font-size: 14px; font-weight: 600; cursor: pointer; }
.button:hover:not(:disabled) { background: #f3f4f6; }
.button:disabled, .add-controls input:disabled { color: #8c959f; cursor: not-allowed; }
.form-hint { margin: 8px 0 0; color: #6e7781; font-size: 12px; }
.primary-card { display: flex; align-items: center; justify-content: space-between; gap: 28px; padding: 22px 24px; border: 1px solid #d0d7de; border-radius: 6px; }
.primary-card h2 { margin: 0; font-size: 16px; }
.primary-card p { margin: 7px 0 0; color: #57606a; font-size: 13px; line-height: 1.5; }
.primary-card select { width: min(360px, 42%); min-height: 38px; padding: 7px 34px 7px 12px; border: 1px solid #d0d7de; border-radius: 6px; background: #fff; color: #24292f; font: inherit; }
@media (max-width: 720px) {
  .email-row { min-height: 0; padding: 18px; }
  .primary-card { align-items: stretch; flex-direction: column; }
  .primary-card select { width: 100%; }
}
@media (max-width: 520px) {
  .add-controls { flex-direction: column; }
  .add-controls .button { width: 100%; }
  .email-heading { gap: 6px; }
}
</style>

<template>
  <div class="settings-view">
    <ProfileSettingsSection
      title-id="login-methods-title"
      title="登录方式"
      description="展示系统支持的登录方式，以及当前账号是否已配置或可用。"
    >
      <div v-if="methodsLoading" class="section-state">
        <span class="spinner" aria-hidden="true"></span>
        正在加载登录方式…
      </div>
      <div v-else-if="methodsError" class="section-state error-state">
        <span>{{ methodsError }}</span>
        <button class="button" type="button" @click="loadLoginMethods">重新加载</button>
      </div>
      <div v-else-if="loginMethods.length" class="settings-list">
        <template v-for="method in loginMethods" :key="methodKey(method)">
          <div class="settings-row">
            <div class="method-icon" aria-hidden="true">
              <Mail v-if="method.type === 'email_otp'" :size="20" />
              <RectangleEllipsis v-else-if="method.type === 'password'" :size="20" />
              <QrCode v-else-if="method.type === 'qr_code'" :size="20" />
              <ThirdPartyProviderIcon v-else :provider="method.provider" :size="20" />
            </div>
            <div class="row-copy">
              <strong>{{ methodTitle(method) }}</strong>
              <span>{{ methodDescription(method) }}</span>
            </div>
            <template v-if="method.type === 'email_otp'">
              <router-link class="button" to="/profile/access/emails">管理邮箱</router-link>
            </template>
            <template v-else-if="method.type === 'password'">
              <button
                v-if="method.available"
                class="button"
                type="button"
                :aria-expanded="passwordExpanded"
                @click="togglePasswordForm"
              >
                {{ passwordExpanded ? '收起' : '修改密码' }}
              </button>
              <button v-else class="button" type="button" disabled title="设置密码功能暂未开放">设置密码</button>
            </template>
            <template v-else-if="method.type === 'third_party'">
              <button v-if="!method.bound" class="button" type="button" @click="bindProvider(method.provider)">绑定</button>
              <button
                v-else
                class="button danger"
                type="button"
				:disabled="Boolean(unbindingProvider) || !canUnbindThirdParty(method.provider)"
				:title="canUnbindThirdParty(method.provider) ? `撤销 ${providerName(method.provider)} 授权` : '至少需要保留一种可用登录方式'"
                @click="unbindProvider(method)"
              >
                {{ unbindingProvider === method.provider ? '撤销中…' : '撤销授权' }}
              </button>
            </template>
          </div>

          <form v-if="method.type === 'password' && method.available && passwordExpanded" class="password-change-form" @submit.prevent="submitPasswordChange">
            <label for="current-password" :class="{ 'field-label-error': passwordErrorField === 'old' }">旧密码</label>
            <div class="password-input-wrapper" :class="{ 'is-error': passwordErrorField === 'old' }">
              <input id="current-password" v-model="passwordForm.oldPassword" type="password" autocomplete="current-password" :aria-describedby="passwordErrorField === 'old' ? 'current-password-error' : undefined" :disabled="passwordSubmitting" @input="clearPasswordError('old')" />
              <TriangleAlert v-if="passwordErrorField === 'old'" :size="20" aria-hidden="true" />
            </div>
            <p v-if="passwordErrorField === 'old'" id="current-password-error" class="password-field-error" role="alert">{{ passwordFormError }}</p>

            <label for="new-password" :class="{ 'field-label-error': passwordErrorField === 'new' }">新密码</label>
            <div class="password-input-wrapper" :class="{ 'is-error': passwordErrorField === 'new' }">
              <input id="new-password" v-model="passwordForm.newPassword" type="password" autocomplete="new-password" :aria-describedby="passwordErrorField === 'new' ? 'new-password-error' : undefined" :disabled="passwordSubmitting" @input="clearPasswordError('new')" />
              <TriangleAlert v-if="passwordErrorField === 'new'" :size="20" aria-hidden="true" />
            </div>
            <p v-if="passwordErrorField === 'new'" id="new-password-error" class="password-field-error" role="alert">{{ passwordFormError }}</p>

            <label for="confirm-new-password" :class="{ 'field-label-error': passwordErrorField === 'confirm' }">确认新密码</label>
            <div class="password-input-wrapper" :class="{ 'is-error': passwordErrorField === 'confirm' }">
              <input id="confirm-new-password" v-model="passwordForm.confirmPassword" type="password" autocomplete="new-password" :aria-describedby="passwordErrorField === 'confirm' ? 'confirm-new-password-error' : undefined" :disabled="passwordSubmitting" @input="clearPasswordError('confirm')" />
              <TriangleAlert v-if="passwordErrorField === 'confirm'" :size="20" aria-hidden="true" />
            </div>
            <p v-if="passwordErrorField === 'confirm'" id="confirm-new-password-error" class="password-field-error" role="alert">{{ passwordFormError }}</p>

            <p class="password-rule" aria-live="polite">密码需为<span :class="passwordRuleClass(passwordLengthValid)">10至256位</span>，且同时包含<span :class="passwordRuleClass(passwordLetterValid)">英文字符</span>和<span :class="passwordRuleClass(passwordDigitValid)">数字</span></p>
            <div class="password-form-actions">
              <button class="button primary" type="submit" :disabled="passwordSubmitting">
                {{ passwordSubmitting ? '更新中…' : '更新密码' }}
              </button>
              <router-link class="forgot-password-link" to="/reset-password">忘记密码？</router-link>
            </div>
          </form>
        </template>
      </div>
      <div v-else class="section-state">当前账号暂无可用登录方式。</div>
    </ProfileSettingsSection>

    <ProfileSettingsSection
      title-id="passkeys-title"
      title="Passkeys"
      description="Passkey 当前用于确认绑定、解绑和其他敏感操作，不作为账号登录方式。"
    >
      <template #actions>
        <button class="button primary" type="button" @click="enrollmentOpen = true">
          <Plus :size="16" />
          注册 Passkey
        </button>
      </template>

      <div v-if="passkeysLoading" class="section-state">
        <span class="spinner" aria-hidden="true"></span>
        正在加载 Passkey…
      </div>
      <div v-else-if="passkeysError" class="section-state error-state">
        <span>{{ passkeysError }}</span>
        <button class="button" type="button" @click="loadPasskeys">重新加载</button>
      </div>
      <div v-else-if="passkeys.length" class="settings-list">
        <div v-for="passkey in passkeys" :key="passkey.id" class="settings-row passkey-row">
          <div class="method-icon" aria-hidden="true"><Fingerprint :size="20" /></div>
          <div class="row-copy">
            <strong>{{ passkey.name }}</strong>
            <span>{{ passkeyState(passkey) }} · 最近使用 {{ formatDate(passkey.last_used_at) }}</span>
          </div>
          <div class="row-actions">
            <button class="icon-button" type="button" title="重命名" @click="renamePasskey(passkey)">
              <Pencil :size="16" />
            </button>
            <button class="icon-button danger-icon" type="button" title="删除" @click="deletePasskey(passkey)">
              <Trash2 :size="16" />
            </button>
          </div>
        </div>
      </div>
      <div v-else class="section-state">尚未注册 Passkey。</div>
    </ProfileSettingsSection>

    <PasskeyEnrollmentDialog
      :visible="enrollmentOpen"
      @close="enrollmentOpen = false"
      @registered="handlePasskeyRegistered"
    />
  </div>
</template>

<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Fingerprint, Mail, Pencil, Plus, QrCode, RectangleEllipsis, Trash2, TriangleAlert } from 'lucide-vue-next'
import { passkeyAPI, userAPI } from '../../api/auth'
import PasskeyEnrollmentDialog from '../../components/PasskeyEnrollmentDialog.vue'
import ProfileSettingsSection from '../../components/profile/ProfileSettingsSection.vue'
import ThirdPartyProviderIcon from '../../components/ThirdPartyProviderIcon.vue'
import { isReauthCancelled } from '../../utils/reauthCoordinator'
import { passwordPolicyError } from '../../utils/passwordPolicy'
import { PROFILE_CONTEXT_KEY } from './profileContext'

const route = useRoute()
const router = useRouter()
const profile = inject(PROFILE_CONTEXT_KEY)
if (!profile) throw new Error('ProfileAuthentication must be rendered inside Profile')

const { user } = profile
const loginMethods = ref([])
const methodsLoading = ref(true)
const methodsError = ref('')
const unbindingProvider = ref('')
const passwordExpanded = ref(false)
const passwordSubmitting = ref(false)
const passwordFormError = ref('')
const passwordErrorField = ref('')
const passwordForm = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})
const passkeys = ref([])
const passkeysLoading = ref(true)
const passkeysError = ref('')
const enrollmentOpen = ref(false)

const providerNames = {
  github: 'GitHub',
  feishu: '飞书'
}

const boundProviderCount = computed(() => loginMethods.value.filter(method => method.type === 'third_party' && method.bound).length)
const canUnbindThirdParty = () => Boolean(user.value?.email?.trim()) || boundProviderCount.value > 1
const passwordLengthValid = computed(() => {
  const length = [...passwordForm.value.newPassword].length
  return length >= 10 && length <= 256
})
const passwordLetterValid = computed(() => /[A-Za-z]/.test(passwordForm.value.newPassword))
const passwordDigitValid = computed(() => /[0-9]/.test(passwordForm.value.newPassword))
const providerName = (provider) => providerNames[provider] || provider
const methodKey = (method) => method.type === 'third_party' ? `${method.type}:${method.provider}` : method.type
const methodTitle = (method) => ({ email_otp: '邮箱验证码', password: '密码', qr_code: '扫码登录' })[method.type] || providerName(method.provider)
const methodDescription = (method) => {
  if (method.type === 'email_otp') return `已设置 ${Number(method.verified_email_count) || 0} 个验证后的邮箱`
  if (method.type === 'password') return method.available ? '已设置，可使用密码登录' : '未设置'
  if (method.type === 'qr_code') return '可通过当前已登录设备扫码确认新设备登录'
  return method.bound ? '已绑定，可用于登录当前账号' : '尚未绑定'
}

const resetPasswordForm = () => {
  passwordForm.value = { oldPassword: '', newPassword: '', confirmPassword: '' }
  passwordFormError.value = ''
  passwordErrorField.value = ''
}

const setPasswordError = (field, message) => {
  passwordErrorField.value = field
  passwordFormError.value = message
}

const clearPasswordError = (field) => {
  if (passwordErrorField.value !== field) return
  passwordErrorField.value = ''
  passwordFormError.value = ''
}

const passwordRuleClass = (isValid) => {
  if (!passwordForm.value.newPassword && passwordErrorField.value !== 'new') return ''
  return isValid ? 'is-valid' : 'is-invalid'
}

const togglePasswordForm = () => {
  passwordExpanded.value = !passwordExpanded.value
  if (!passwordExpanded.value) resetPasswordForm()
}

const submitPasswordChange = async () => {
  if (passwordSubmitting.value) return
  if (!passwordForm.value.oldPassword) {
    setPasswordError('old', '请输入旧密码')
    return
  }
  const policyError = passwordPolicyError(passwordForm.value.newPassword)
  if (policyError) {
    setPasswordError('new', policyError)
    return
  }
  if (passwordForm.value.newPassword !== passwordForm.value.confirmPassword) {
    setPasswordError('confirm', '两次输入的新密码不一致')
    return
  }

  passwordSubmitting.value = true
  clearPasswordError(passwordErrorField.value)
  try {
    await userAPI.changePassword({
      old_password: passwordForm.value.oldPassword,
      new_password: passwordForm.value.newPassword
    })
    passwordExpanded.value = false
    resetPasswordForm()
    ElMessage.success('密码已更新，其他设备需要重新登录')
  } catch (error) {
    if (error.status === 401) {
      redirectToLogin()
      return
    }
    const message = error.message || '修改密码失败'
    if (message === '需为10至256位' || message === '需包含英文字符' || message === '需包含数字') {
      setPasswordError('new', message)
    } else if (message === '旧密码错误') {
      setPasswordError('old', message)
    } else {
      setPasswordError('confirm', message)
    }
  } finally {
    passwordSubmitting.value = false
  }
}

const redirectToLogin = () => {
  router.replace({ path: '/login', query: { redirect: route.fullPath } })
}

const loadLoginMethods = async () => {
  methodsLoading.value = true
  methodsError.value = ''
  try {
    const result = await userAPI.getLoginMethods()
    loginMethods.value = Array.isArray(result?.data?.methods) ? result.data.methods : []
  } catch (error) {
    if (error.status === 401) {
      redirectToLogin()
      return
    }
    loginMethods.value = []
    methodsError.value = error.message || '登录方式加载失败'
  } finally {
    methodsLoading.value = false
  }
}

const loadPasskeys = async () => {
  passkeysLoading.value = true
  passkeysError.value = ''
  try {
    const result = await passkeyAPI.list()
    passkeys.value = Array.isArray(result?.data?.passkeys) ? result.data.passkeys : []
  } catch (error) {
    if (error.status === 401) {
      redirectToLogin()
      return
    }
    passkeys.value = []
    passkeysError.value = error.message || 'Passkey 加载失败'
  } finally {
    passkeysLoading.value = false
  }
}

const bindProvider = (provider) => {
  router.push({ name: 'ThirdPartyBindPreview', query: { provider } })
}

const unbindProvider = async (method) => {
  if (unbindingProvider.value || !canUnbindThirdParty(method.provider)) return
  const name = providerName(method.provider)
  try {
    await ElMessageBox.confirm(
      `撤销后将无法再使用 ${name} 登录当前账号，确定继续吗？`,
      `撤销 ${name} 授权`,
      { confirmButtonText: '撤销授权', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }

  unbindingProvider.value = method.provider
  try {
    await userAPI.unbindThirdParty(method.provider)
    method.bound = false
    method.available = false
    ElMessage.success(`${name} 授权已撤销`)
  } catch (error) {
    if (isReauthCancelled(error)) return
    if (error.status === 401) {
      redirectToLogin()
      return
    }
    ElMessage.error(error.message || '撤销授权失败')
  } finally {
    unbindingProvider.value = ''
  }
}

const handlePasskeyRegistered = async () => {
  enrollmentOpen.value = false
  await loadPasskeys()
  ElMessage.success('Passkey 注册成功')
}

const renamePasskey = async (passkey) => {
  try {
    const { value } = await ElMessageBox.prompt('输入便于识别的名称', '重命名 Passkey', {
      inputValue: passkey.name,
      inputValidator: (name) => Boolean(name?.trim()) || '名称不能为空'
    })
    await passkeyAPI.rename(passkey.id, value.trim())
    passkey.name = value.trim()
    ElMessage.success('Passkey 名称已更新')
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error.message || '重命名失败')
  }
}

const deletePasskey = async (passkey) => {
  const isLast = passkeys.value.length === 1
  try {
    await ElMessageBox.confirm(
      isLast
        ? '这是最后一个 Passkey。删除后，第三方账号管理等敏感操作将被阻断，直到你通过邮箱验证码重新注册 Passkey。'
        : `确定删除“${passkey.name}”吗？`,
      '删除 Passkey',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
    await passkeyAPI.remove(passkey.id)
    passkeys.value = passkeys.value.filter((item) => item.id !== passkey.id)
    ElMessage.success('Passkey 已删除')
  } catch (error) {
    if (error === 'cancel' || error === 'close' || isReauthCancelled(error)) return
    ElMessage.error(error.message || '删除失败')
  }
}

const passkeyState = (passkey) => passkey.backup_state ? '已同步' : (passkey.backup_eligible ? '可同步' : '仅此设备')
const formatDate = (value) => {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'
  }).format(new Date(value))
}

onMounted(() => {
  if (route.query.bind === 'success') {
    ElMessage.success('第三方账号绑定成功')
    router.replace({ path: route.path })
  } else if (route.query.bind_error) {
    ElMessage.error(String(route.query.bind_error))
    router.replace({ path: route.path })
  }
  loadLoginMethods()
  loadPasskeys()
})
</script>

<style scoped>
.settings-view {
  display: grid;
  gap: 34px;
}

.settings-list {
  overflow: hidden;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
}

.settings-row {
  display: grid;
  min-height: 70px;
  box-sizing: border-box;
  grid-template-columns: 34px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-top: 1px solid var(--profile-divider);
}

.settings-row:first-child {
  border-top: 0;
}

.password-change-form {
  display: grid;
  gap: 8px;
  padding: 6px 16px 20px 62px;
}

.password-change-form label {
  color: var(--profile-text-strong);
  font-size: 14px;
  font-weight: 600;
}

.password-change-form .field-label-error {
  color: var(--profile-danger);
}

.password-input-wrapper {
  position: relative;
  width: min(100%, 420px);
}

.password-change-form input {
  width: 100%;
  min-height: 34px;
  box-sizing: border-box;
  padding: 6px 10px;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
  color: var(--profile-text-strong);
  font: inherit;
}

.password-input-wrapper > svg {
  position: absolute;
  top: 50%;
  right: 10px;
  color: var(--profile-danger);
  pointer-events: none;
  transform: translateY(-50%);
}

.password-input-wrapper.is-error input {
  border-color: var(--profile-danger);
}

.password-change-form input:focus {
  border-color: var(--profile-accent);
  outline: none;
  box-shadow: 0 0 0 3px var(--profile-focus-ring);
}

.password-input-wrapper.is-error input:focus {
  border-color: var(--profile-danger);
  box-shadow: 0 0 0 3px var(--profile-danger-focus-ring);
}

.password-field-error {
  position: relative;
  width: min(100%, 420px);
  box-sizing: border-box;
  margin: 0;
  padding: 9px 12px;
  border: 1px solid var(--profile-danger-border);
  border-radius: 6px;
  background: var(--profile-danger-soft);
  color: var(--profile-danger);
  font-size: 13px;
  line-height: 1.45;
}

.password-field-error::before {
  position: absolute;
  top: -6px;
  left: 16px;
  width: 10px;
  height: 10px;
  border-top: 1px solid var(--profile-danger-border);
  border-left: 1px solid var(--profile-danger-border);
  background: var(--profile-danger-soft);
  content: '';
  transform: rotate(45deg);
}

.password-rule {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.45;
  color: var(--profile-text-muted);
}

.password-rule .is-valid {
  color: var(--profile-success);
  font-weight: 600;
}

.password-rule .is-invalid {
  color: var(--profile-danger);
  font-weight: 600;
}

.password-form-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 4px;
}

.forgot-password-link {
  color: var(--profile-accent);
  font-size: 14px;
  text-decoration: none;
}

.forgot-password-link:hover {
  text-decoration: underline;
}

.method-icon {
  display: flex;
  width: 30px;
  height: 30px;
  align-items: center;
  justify-content: center;
  color: var(--profile-text-strong);
}

.row-copy {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.row-copy strong {
  font-size: 14px;
}

.row-copy span {
  color: var(--profile-text-muted);
  font-size: 13px;
  line-height: 1.45;
}

.row-actions {
  display: flex;
  gap: 6px;
}

.section-state {
  display: flex;
  min-height: 84px;
  box-sizing: border-box;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
  color: var(--profile-text-muted);
  font-size: 14px;
  padding: 16px;
}

.error-state {
  justify-content: space-between;
  color: var(--profile-danger);
}

.spinner {
  width: 17px;
  height: 17px;
  border: 2px solid var(--profile-border);
  border-top-color: var(--profile-accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.button,
.icon-button {
  display: inline-flex;
  min-height: 32px;
  box-sizing: border-box;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1px solid var(--profile-border-muted);
  border-radius: 6px;
  background: var(--profile-surface-subtle);
  color: var(--profile-text-strong);
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
}

.button {
  padding: 5px 12px;
}

.button.primary {
  border-color: var(--profile-success-strong);
  background: var(--profile-success-strong);
  color: #ffffff;
}

.button.danger,
.danger-icon {
  color: var(--profile-danger);
}

.button:hover:not(:disabled),
.icon-button:hover:not(:disabled) {
  background: var(--profile-surface-hover);
}

.button.primary:hover:not(:disabled) {
  background: var(--profile-success);
}

.button:disabled,
.icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.icon-button {
  width: 32px;
  flex: 0 0 32px;
  padding: 0;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 620px) {
  .settings-row {
    grid-template-columns: 30px minmax(0, 1fr);
  }

  .settings-row > .button,
  .row-actions {
    grid-column: 2;
    justify-self: start;
  }

  .password-change-form {
    padding-left: 16px;
  }
}
</style>

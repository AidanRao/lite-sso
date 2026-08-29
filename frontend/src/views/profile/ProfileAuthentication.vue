<template>
  <div class="settings-view">
    <section class="settings-section" aria-labelledby="login-methods-title">
      <header class="section-title section-title-divided">
        <h2 id="login-methods-title">登录方式</h2>
      </header>
      <p class="section-copy">系统只展示当前账号实际可用或可以绑定的方式。</p>

      <div v-if="methodsLoading" class="section-state">
        <span class="spinner" aria-hidden="true"></span>
        正在加载登录方式…
      </div>
      <div v-else-if="methodsError" class="section-state error-state">
        <span>{{ methodsError }}</span>
        <button class="button" type="button" @click="loadLoginMethods">重新加载</button>
      </div>
      <div v-else-if="loginMethods.length" class="settings-list">
        <div v-for="method in loginMethods" :key="methodKey(method)" class="settings-row">
          <div class="method-icon" aria-hidden="true">
            <Mail v-if="method.type === 'email_otp'" :size="20" />
            <RectangleEllipsis v-else-if="method.type === 'password'" :size="20" />
            <ThirdPartyProviderIcon v-else :provider="method.provider" :size="20" />
          </div>
          <div class="row-copy">
            <strong>{{ methodTitle(method) }}</strong>
            <span>{{ methodDescription(method) }}</span>
          </div>
          <template v-if="method.type === 'third_party'">
            <button v-if="!method.bound" class="button" type="button" @click="bindProvider(method.provider)">绑定</button>
            <button
              v-else
              class="button danger"
              type="button"
              :disabled="Boolean(unbindingProvider) || !canUnbindThirdParty"
              :title="canUnbindThirdParty ? `撤销 ${providerName(method.provider)} 授权` : '请先设置邮箱，再撤销第三方授权'"
              @click="unbindProvider(method)"
            >
              {{ unbindingProvider === method.provider ? '撤销中…' : '撤销授权' }}
            </button>
          </template>
        </div>
      </div>
      <div v-else class="section-state">当前账号暂无可用登录方式。</div>
    </section>

    <section class="settings-section" aria-labelledby="passkeys-title">
      <header class="section-title section-title-actions section-title-divided">
        <h2 id="passkeys-title">Passkeys</h2>
        <button class="button primary" type="button" @click="enrollmentOpen = true">
          <Plus :size="16" />
          注册 Passkey
        </button>
      </header>
      <p class="section-copy">Passkey 当前用于确认绑定、解绑和其他敏感操作，不作为账号登录方式。</p>

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
    </section>

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
import { Fingerprint, Mail, Pencil, Plus, RectangleEllipsis, Trash2 } from 'lucide-vue-next'
import { passkeyAPI, userAPI } from '../../api/auth'
import PasskeyEnrollmentDialog from '../../components/PasskeyEnrollmentDialog.vue'
import ThirdPartyProviderIcon from '../../components/ThirdPartyProviderIcon.vue'
import { isReauthCancelled } from '../../utils/reauthCoordinator'
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
const passkeys = ref([])
const passkeysLoading = ref(true)
const passkeysError = ref('')
const enrollmentOpen = ref(false)

const providerNames = {
  github: 'GitHub',
  feishu: '飞书'
}

const canUnbindThirdParty = computed(() => Boolean(user.value?.email?.trim()))
const providerName = (provider) => providerNames[provider] || provider
const methodKey = (method) => method.type === 'third_party' ? `${method.type}:${method.provider}` : method.type
const methodTitle = (method) => ({ email_otp: '邮箱验证码', password: '密码' })[method.type] || providerName(method.provider)
const methodDescription = (method) => {
  if (method.type === 'email_otp') return `可向 ${method.email} 发送登录验证码`
  if (method.type === 'password') return '已为当前账号配置密码'
  return method.bound ? '已绑定，可用于登录当前账号' : '尚未绑定'
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
  if (unbindingProvider.value || !canUnbindThirdParty.value) return
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

.section-title h2 {
  margin: 0;
  color: #24292f;
  font-size: 18px;
  font-weight: 600;
}

.settings-section {
  display: grid;
  gap: 14px;
}

.section-title-divided {
  padding-bottom: 8px;
  border-bottom: 1px solid #d8dee4;
}

.section-title-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.section-copy {
  margin: 0;
  color: #57606a;
  font-size: 14px;
  line-height: 1.5;
}

.settings-list {
  overflow: hidden;
  border: 1px solid #d0d7de;
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
  border-top: 1px solid #d8dee4;
}

.settings-row:first-child {
  border-top: 0;
}

.method-icon {
  display: flex;
  width: 30px;
  height: 30px;
  align-items: center;
  justify-content: center;
  color: #24292f;
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
  color: #57606a;
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
  border: 1px solid #d0d7de;
  border-radius: 6px;
  color: #57606a;
  font-size: 14px;
  padding: 16px;
}

.error-state {
  justify-content: space-between;
  color: #cf222e;
}

.spinner {
  width: 17px;
  height: 17px;
  border: 2px solid #d0d7de;
  border-top-color: #0969da;
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
  border: 1px solid rgba(27, 31, 36, 0.15);
  border-radius: 6px;
  background: #f6f8fa;
  color: #24292f;
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
}

.button {
  padding: 5px 12px;
}

.button.primary {
  border-color: #1f883d;
  background: #1f883d;
  color: #ffffff;
}

.button.danger,
.danger-icon {
  color: #cf222e;
}

.button:hover:not(:disabled),
.icon-button:hover:not(:disabled) {
  background: #eaeef2;
}

.button.primary:hover:not(:disabled) {
  background: #1a7f37;
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
  .section-title-actions {
    align-items: flex-start;
    flex-direction: column;
  }

  .settings-row {
    grid-template-columns: 30px minmax(0, 1fr);
  }

  .settings-row > .button,
  .row-actions {
    grid-column: 2;
    justify-self: start;
  }
}
</style>

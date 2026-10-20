<template>
  <section class="client-form-page" :aria-labelledby="detail ? undefined : 'client-form-title'">
    <RouterLink v-if="!detail" class="form-back" to="/admin/clients"><ArrowLeft :size="14" />客户端管理</RouterLink>
    <header v-if="!detail" class="page-heading form-heading">
      <div><h1 id="client-form-title">{{ isEditing ? '编辑 OAuth 客户端' : '新增 OAuth 客户端' }}</h1><p>配置应用信息和授权回调，接入统一身份认证。</p></div>
    </header>
    <form class="client-form" @submit.prevent="saveClient">
      <div v-if="saveError" class="admin-error" role="alert">{{ saveError }}</div>
      <fieldset :disabled="saving">
        <div class="form-section basic-fields">
          <label for="client-name">应用名称 <span class="required-mark">*</span></label>
          <input id="client-name" v-model.trim="form.name" required maxlength="50" autocomplete="off" aria-describedby="client-name-help" />
          <small id="client-name-help">用户在登录和授权时看到的应用名称。</small>

          <label for="client-homepage">主页地址 <span class="required-mark">*</span></label>
          <input id="client-homepage" v-model.trim="form.homepage_url" required maxlength="255" placeholder="https://example.com" inputmode="url" aria-describedby="client-homepage-help" />
          <small id="client-homepage-help">应用首页的完整地址。</small>

          <div class="form-columns">
            <div v-if="!detail" class="form-field">
              <label for="client-id">Client ID <span class="required-mark">*</span></label>
              <input v-if="!currentClient" id="client-id" v-model.trim="form.client_id" required maxlength="50" autocomplete="off" spellcheck="false" aria-describedby="client-id-help" />
              <output v-else id="client-id" class="readonly-output mono">{{ form.client_id }}</output>
              <small id="client-id-help">{{ currentClient ? '客户端标识创建后不可修改。' : '用于标识接入应用，创建后不可修改。' }}</small>
            </div>
            <div class="form-field">
              <label for="client-type">客户端类型</label>
              <SelectMenu id="client-type" v-model="form.client_type" label="客户端类型" :options="clientTypeOptions" :disabled="saving" @change="handleClientTypeChange" />
              <small>{{ form.client_type === 'public' ? '原生应用或浏览器应用，使用 PKCE 授权。' : '由应用后端保管密钥并兑换令牌。' }}</small>
            </div>
          </div>
        </div>

        <section class="form-section" aria-labelledby="redirects-title">
          <header class="form-section-heading">
            <h2 id="redirects-title">回调地址 <span class="required-mark">*</span></h2>
            <button class="admin-button" type="button" @click="addRedirect"><Plus :size="14" />添加回调地址</button>
          </header>
          <p class="field-description">授权完成后跳转到的地址。至少配置一个，授权请求中的 redirect_uri 必须与其中一项完全一致。</p>
          <div v-for="(row, index) in redirectRows" :key="row.key" class="redirect-field">
            <label :for="`redirect-${row.key}`">回调地址 {{ index + 1 }}</label>
            <div class="redirect-input-row">
              <input :id="`redirect-${row.key}`" v-model.trim="row.value" required inputmode="url" placeholder="https://example.com/auth/callback" spellcheck="false" />
              <button class="admin-button danger-text" type="button" :aria-label="`删除回调地址 ${index + 1}`" :disabled="redirectRows.length === 1" @click="removeRedirect(index)"><Trash2 :size="14" /><span>删除</span></button>
            </div>
          </div>
        </section>

        <section class="form-section" aria-labelledby="permissions-title">
          <h2 id="permissions-title">访问权限</h2>
          <div class="form-columns">
            <div class="form-field">
              <label for="client-audiences">API Audience</label>
              <textarea id="client-audiences" v-model="form.audiences" rows="3" placeholder="classhopper-api" spellcheck="false" aria-describedby="client-audiences-help" />
              <small id="client-audiences-help">每行一个。至少配置一个才能签发令牌。</small>
            </div>
            <div class="form-field">
              <label for="client-scopes">允许的 Scope</label>
              <textarea id="client-scopes" v-model="form.allowed_scopes" rows="3" placeholder="courses:read" spellcheck="false" aria-describedby="client-scopes-help" />
              <small id="client-scopes-help">每行一个。未传 scope 时签发全部已配置权限。</small>
            </div>
          </div>
          <p class="field-description">qr:login 允许此应用确认浏览器扫码登录，需同时配置 sso-api Audience。撤销扫码权限将立即阻止未完成的登录。</p>
        </section>

        <section v-if="!detail && form.client_type === 'confidential'" class="form-section" aria-labelledby="secret-title">
          <h2 id="secret-title">Client Secret</h2>
          <p class="field-description">仅由应用后端保管，请勿放入浏览器或原生应用。</p>
          <div class="secret-row">
            <output class="secret-output mono" aria-label="Client Secret">{{ secretDisplay }}</output>
            <button class="icon-button" type="button" title="复制密钥" aria-label="复制密钥" :disabled="!form.client_secret" @click="copyClientSecret"><Copy :size="16" /></button>
            <button class="icon-button" type="button" :title="secretVisible ? '隐藏密钥' : '显示密钥'" :aria-label="secretVisible ? '隐藏密钥' : '显示密钥'" :disabled="!form.client_secret" @click="secretVisible = !secretVisible"><EyeOff v-if="secretVisible" :size="16" /><Eye v-else :size="16" /></button>
            <button class="icon-button" type="button" title="重新生成密钥" aria-label="重新生成密钥"  @click="regenerateClientSecret"><RefreshCw :size="16" /></button>
          </div>
        </section>

        <section class="form-section" aria-labelledby="optional-title">
          <h2 id="optional-title">其他设置</h2>
          <div class="form-field">
            <label for="client-logout">登出通知地址 <span class="optional-mark">可选</span></label>
            <input id="client-logout" v-model.trim="form.logout_uri" maxlength="255" placeholder="https://example.com/auth/logout" inputmode="url" />
          </div>
          <div class="form-field">
            <span class="field-label">应用 Logo <span class="optional-mark">可选</span></span>
            <div class="logo-editor">
              <ApplicationLogo :label="form.name || form.client_id" :src="displayedLogoURL" size="large" />
              <div class="logo-actions">
                <button class="admin-button" type="button" @click="logoInput?.click()"><Upload :size="14" />{{ displayedLogoURL ? '更换 Logo' : '上传 Logo' }}</button>
                <button v-if="displayedLogoURL" class="admin-button danger-text" type="button" @click="clearClientLogo">清除</button>
                <small>JPEG、PNG、WebP，最大 1MB。</small>
              </div>
            </div>
            <input ref="logoInput" class="logo-file-input" type="file" accept="image/jpeg,image/png,image/webp" aria-label="选择应用 Logo" @change="selectClientLogo" />
          </div>
        </section>
        <footer class="form-actions">
          <button class="admin-button primary" type="submit" >{{ saving ? '正在保存…' : isEditing ? '保存更改' : currentClient ? '完成创建' : '创建客户端' }}</button>
          <button v-if="!detail" class="admin-button plain" type="button" @click="router.push('/admin/clients')">取消</button>
        </footer>
      </fieldset>
    </form>
  </section>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Copy, Eye, EyeOff, Plus, RefreshCw, Trash2, Upload } from 'lucide-vue-next'
import { adminAPI } from '../../api/auth'
import SelectMenu from '../../components/SelectMenu.vue'
import ApplicationLogo from '../../components/ApplicationLogo.vue'
import { generateClientSecret } from '../../utils/clientSecret'
import { useAdminAccess } from './useAdminAccess'

const props = defineProps({ detail: Boolean, client: { type: Object, default: null } })
const emit = defineEmits(['saved'])
const router = useRouter()
const { handleAccessError } = useAdminAccess()
const clientTypeOptions = [
  { value: 'confidential', label: '服务端客户端' },
  { value: 'public', label: '公共客户端 · PKCE' }
]
const currentClient = ref(null)
const isEditing = computed(() => props.detail)
const saving = ref(false)
const saveError = ref('')
const secretVisible = ref(false)
const logoInput = ref(null)
const pendingLogoFile = ref(null)
const logoPreviewURL = ref('')
let redirectKey = 0
let disposed = false
const newRedirect = (value = '') => ({ key: ++redirectKey, value })
const redirectRows = ref([newRedirect()])
const form = reactive({ name: '', client_id: '', client_type: 'confidential', client_secret: '', homepage_url: '', audiences: '', allowed_scopes: '', logout_uri: '' })
const secretDisplay = computed(() => secretVisible.value ? form.client_secret : 'xxx')
const displayedLogoURL = computed(() => logoPreviewURL.value || currentClient.value?.logo_url || '')
const notify = message => ElMessage.success({ message, customClass: 'admin-toast' })

const regenerateClientSecret = () => {
  form.client_secret = generateClientSecret()
  secretVisible.value = false
}
const handleClientTypeChange = () => {
  secretVisible.value = false
  form.client_secret = ''
  if (props.detail || form.client_type === 'public') return
  regenerateClientSecret()
}
const loadClient = () => {
  if (!props.detail) { regenerateClientSecret(); return }
  const client = props.client
  currentClient.value = client
  Object.assign(form, { name: client.name, client_id: client.client_id, client_type: client.client_type, homepage_url: client.homepage_url, logout_uri: client.logout_uri || '', audiences: (client.audiences || []).join('\n'), allowed_scopes: (client.allowed_scopes || []).join('\n') })
  redirectRows.value = client.redirect_uris?.length ? client.redirect_uris.map(newRedirect) : [newRedirect()]
}
const addRedirect = async () => {
  const row = newRedirect()
  redirectRows.value.push(row)
  await nextTick()
  document.getElementById(`redirect-${row.key}`)?.focus()
}
const removeRedirect = async index => {
  if (redirectRows.value.length === 1) return
  redirectRows.value.splice(index, 1)
  await nextTick()
  const row = redirectRows.value[Math.min(index, redirectRows.value.length - 1)]
  document.getElementById(`redirect-${row.key}`)?.focus()
}
const clearPendingLogo = () => {
  if (logoPreviewURL.value) URL.revokeObjectURL(logoPreviewURL.value)
  logoPreviewURL.value = ''
  pendingLogoFile.value = null
}
const selectClientLogo = event => {
  const [file] = event.target.files || []
  event.target.value = ''
  if (!file) return
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || file.size > 1024 * 1024) {
    saveError.value = '请选择不超过 1MB 的 JPEG、PNG 或 WebP 图片。'
    return
  }
  clearPendingLogo()
  pendingLogoFile.value = file
  logoPreviewURL.value = URL.createObjectURL(file)
}
const clearClientLogo = async () => {
  if (pendingLogoFile.value) { clearPendingLogo(); return }
  if (!currentClient.value?.logo_url || saving.value) return
  saving.value = true
  saveError.value = ''
  try {
    const response = await adminAPI.clearOAuthClientLogo(currentClient.value.id)
    currentClient.value = response?.data?.client || { ...currentClient.value, logo_url: null }
    emit('saved', currentClient.value)
    notify('应用 Logo 已清除')
  } catch (error) {
    if (!handleAccessError(error)) saveError.value = error.message || '清除 Logo 失败'
  } finally {
    saving.value = false
  }
}
const copyClientSecret = async () => {
  try { await navigator.clipboard.writeText(form.client_secret); notify('密钥已复制') }
  catch { saveError.value = '复制失败，请显示密钥后手动复制。' }
}
const saveClient = async () => {
  if (saving.value) return
  saving.value = true
  saveError.value = ''
  let savedClient = null
  const wasEditing = isEditing.value
  try {
    const lines = value => value.split('\n').map(item => item.trim()).filter(Boolean)
    const payload = { name: form.name, homepage_url: form.homepage_url, client_type: form.client_type, audiences: lines(form.audiences), allowed_scopes: lines(form.allowed_scopes), redirect_uris: redirectRows.value.map(row => row.value.trim()), logout_uri: form.logout_uri }
    if (!currentClient.value && form.client_type === 'confidential') payload.client_secret = form.client_secret
    let response
    if (currentClient.value) response = await adminAPI.updateOAuthClient(currentClient.value.id, payload)
    else response = await adminAPI.createOAuthClient({ ...payload, client_id: form.client_id })
    savedClient = response?.data?.client
    currentClient.value = savedClient
    if (pendingLogoFile.value) {
      const logoResponse = await adminAPI.uploadOAuthClientLogo(savedClient.id, pendingLogoFile.value)
      savedClient = logoResponse.data.client
      currentClient.value = savedClient
      clearPendingLogo()
    }
    if (disposed) return
    notify(wasEditing ? '客户端已更新' : '客户端已创建')
    if (props.detail) emit('saved', savedClient)
    else await router.push(`/admin/clients/${savedClient.id}`)
  } catch (error) {
    if (disposed || handleAccessError(error)) return
    saveError.value = savedClient ? '客户端信息已保存，但 Logo 上传失败。请重试保存以完成上传。' : error.message || '保存客户端失败'
    await nextTick()
    document.querySelector('.client-form .admin-error')?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  } finally {
    saving.value = false
  }
}
onMounted(loadClient)
onBeforeUnmount(() => { disposed = true; clearPendingLogo() })
</script>

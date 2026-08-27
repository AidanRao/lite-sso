<template>
  <main class="binding-page">
    <section class="binding-card" aria-labelledby="binding-title">
      <button class="back-button" type="button" :disabled="confirming || startingAuthorization" @click="cancel">
        <ArrowLeft :size="18" />
        返回账号页
      </button>

      <div v-if="loading" class="state-panel">
        <span class="loading-spinner" aria-hidden="true"></span>
        <p>正在读取账号信息…</p>
      </div>

      <div v-else-if="errorMessage" class="state-panel error-panel">
        <h1 id="binding-title">无法继续绑定</h1>
        <p>{{ errorMessage }}</p>
        <div class="actions error-actions">
          <button v-if="selectedProvider" class="secondary-button" type="button" @click="retryAuthorization">重新授权</button>
          <button class="primary-button" type="button" @click="cancel">返回账号页</button>
        </div>
      </div>

      <template v-else-if="account && selectedProvider">
        <nav class="progress" aria-label="绑定进度">
          <div class="progress-step" :class="{ active: currentStep === 1, completed: currentStep === 2 }">
            <span class="step-number">{{ currentStep === 2 ? '✓' : '1' }}</span>
            <strong>授权 {{ providerName }}</strong>
          </div>
          <span class="progress-line" aria-hidden="true"></span>
          <div class="progress-step" :class="{ active: currentStep === 2 }">
            <span class="step-number">2</span>
            <strong>确认绑定</strong>
          </div>
        </nav>

        <header class="binding-header">
          <p class="eyebrow">第三方账号绑定</p>
          <h1 id="binding-title">{{ currentStep === 1 ? `授权绑定 ${providerName}` : `确认绑定 ${providerName}` }}</h1>
          <p class="description">
            {{ currentStep === 1
              ? `系统将跳转至 ${providerName} 进行授权。授权完成后，请确认将要绑定的账号资料。`
              : '请确认以下第三方账号资料属于你。确认后，该账号将可以用于登录 Lite SSO。' }}
          </p>
        </header>

        <section class="account-comparison" aria-label="绑定账号信息">
          <article class="identity-card">
            <span class="identity-label">当前账号</span>
            <div class="identity-content">
              <div class="avatar" aria-hidden="true">
                <img v-if="account.avatar_url" :src="account.avatar_url" alt="" />
                <span v-else>{{ accountInitial }}</span>
              </div>
              <div class="identity-copy">
                <strong>{{ accountName }}</strong>
                <span>{{ account.email || '未设置邮箱' }}</span>
              </div>
            </div>
          </article>

          <div class="binding-connector" aria-hidden="true">
            <span></span>
            <div><ArrowRightLeft :size="21" /></div>
            <span></span>
          </div>

          <article class="identity-card">
            <span class="identity-label">{{ currentStep === 1 ? '第三方平台' : '第三方账号' }}</span>
            <div class="identity-content">
              <div class="avatar provider-avatar" aria-hidden="true">
                <img v-if="preview?.avatar_url" :src="preview.avatar_url" alt="" />
                <ThirdPartyProviderIcon v-else-if="currentStep === 1" :provider="selectedProvider" :size="31" />
                <span v-else>{{ providerAccountInitial }}</span>
              </div>
              <div class="identity-copy">
                <strong>
                  <ThirdPartyProviderIcon v-if="currentStep === 2" :provider="selectedProvider" :size="20" />
                  {{ currentStep === 1 ? providerName : (preview.username || '未提供用户名') }}
                </strong>
                <span>{{ currentStep === 1 ? `即将跳转至 ${providerName} 完成授权` : (preview.email || '该账号未提供邮箱') }}</span>
              </div>
            </div>
          </article>
        </section>

        <p class="binding-summary">
          <ShieldCheck :size="18" />
          {{ currentStep === 1 ? `授权完成后，可使用 ${providerName} 登录当前 Lite SSO 账号` : `确认后，可使用 ${providerName} 登录当前 Lite SSO 账号` }}
        </p>

        <aside class="notice">
          <Info :size="22" />
          <p>系统仅绑定已验证的第三方身份信息，不会同步密码。<br />绑定后可在账号设置中随时解除绑定。</p>
        </aside>

        <div class="actions">
          <button class="secondary-button" type="button" :disabled="confirming || startingAuthorization" @click="cancel">取消</button>
          <button
            v-if="currentStep === 1"
            class="primary-button"
            type="button"
            :disabled="startingAuthorization"
            @click="startAuthorization"
          >
            {{ startingAuthorization ? '正在前往授权…' : `前往 ${providerName} 授权` }}
          </button>
          <button
            v-else
            class="primary-button"
            type="button"
            :disabled="confirming"
            @click="confirm"
          >
            {{ confirming ? '绑定中…' : `确认绑定 ${providerName}` }}
          </button>
        </div>
      </template>
    </section>
  </main>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRightLeft, Info, ShieldCheck } from 'lucide-vue-next'
import { userAPI } from '../api/auth'
import ThirdPartyProviderIcon from '../components/ThirdPartyProviderIcon.vue'

const route = useRoute()
const router = useRouter()
const bindingID = computed(() => String(route.query.binding_id || ''))
const requestedProvider = computed(() => String(route.query.provider || ''))
const loading = ref(true)
const confirming = ref(false)
const startingAuthorization = ref(false)
const errorMessage = ref('')
const account = ref(null)
const preview = ref(null)

const providerNames = {
  github: 'GitHub',
  feishu: '飞书'
}
const selectedProvider = computed(() => {
  const provider = preview.value?.provider || requestedProvider.value
  return providerNames[provider] ? provider : ''
})
const currentStep = computed(() => preview.value ? 2 : 1)
const providerName = computed(() => providerNames[selectedProvider.value] || '第三方账号')
const accountName = computed(() => account.value?.username || account.value?.email || 'Lite SSO 用户')
const accountInitial = computed(() => accountName.value.slice(0, 1).toUpperCase())
const providerAccountInitial = computed(() => (preview.value?.username || providerName.value).slice(0, 1).toUpperCase())

const cancel = () => {
  router.replace('/profile')
}

const loadBindingContext = async () => {
  if (route.query.error) {
    errorMessage.value = String(route.query.error)
    loading.value = false
    return
  }

  if (!bindingID.value && !selectedProvider.value) {
    errorMessage.value = '请选择要绑定的第三方平台后重新发起授权。'
    loading.value = false
    return
  }

  try {
    const profileResult = await userAPI.getProfile()
    account.value = profileResult?.data?.user || null
    if (!account.value?.id) {
      errorMessage.value = '无法读取当前账号信息，请返回账号页后重试。'
      return
    }

    if (!bindingID.value) {
      return
    }

    const previewResult = await userAPI.getThirdPartyBindingPreview(bindingID.value)
    preview.value = previewResult?.data || null
    if (!selectedProvider.value) {
      errorMessage.value = '绑定预览不存在或已过期，请重新发起绑定。'
    }
  } catch (error) {
    if (error.status === 401) {
      router.replace(`/login?redirect=${encodeURIComponent(route.fullPath)}`)
      return
    }
    errorMessage.value = error.message || '无法读取绑定信息，请重新发起绑定。'
  } finally {
    loading.value = false
  }
}

const startAuthorization = () => {
  if (!selectedProvider.value || startingAuthorization.value) {
    return
  }

  startingAuthorization.value = true
  const redirect = encodeURIComponent('/profile?bind=success')
  window.location.assign(`/api/user/third/${encodeURIComponent(selectedProvider.value)}/bind?redirect=${redirect}`)
}

const retryAuthorization = () => {
  if (!selectedProvider.value) {
    return
  }
  router.replace({
    name: 'ThirdPartyBindPreview',
    query: { provider: selectedProvider.value }
  })
  errorMessage.value = ''
  preview.value = null
  loading.value = false
}

const confirm = async () => {
  if (confirming.value || !bindingID.value) {
    return
  }

  confirming.value = true
  try {
    const result = await userAPI.confirmThirdPartyBinding(bindingID.value)
    router.replace(result?.data?.redirect_url || '/profile?bind=success')
  } catch (error) {
    errorMessage.value = error.message || '确认绑定失败，请重新授权。'
    preview.value = null
  } finally {
    confirming.value = false
  }
}

onMounted(loadBindingContext)
</script>

<style scoped>
.binding-page {
  align-items: center;
  background:
    radial-gradient(circle at 50% 5%, rgba(14, 165, 233, 0.12), transparent 30%),
    #f8fafc;
  box-sizing: border-box;
  display: flex;
  justify-content: center;
  min-height: 100vh;
  padding: 40px 24px;
}

.binding-card {
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid #dbe4ef;
  border-radius: 18px;
  box-shadow: 0 24px 56px rgba(15, 23, 42, 0.11);
  box-sizing: border-box;
  max-width: 1080px;
  min-height: 680px;
  padding: 32px 46px 38px;
  width: 100%;
}

.back-button,
.secondary-button,
.primary-button {
  align-items: center;
  border-radius: 9px;
  cursor: pointer;
  display: inline-flex;
  font: inherit;
  font-weight: 650;
  gap: 7px;
  justify-content: center;
}

.back-button {
  background: transparent;
  border: 0;
  color: #334e70;
  font-size: 16px;
  margin: -6px 0 20px -8px;
  padding: 7px 8px;
}

.back-button:hover:not(:disabled) {
  color: #0284c7;
}

.progress {
  align-items: center;
  display: flex;
  justify-content: center;
  margin: 2px auto 36px;
}

.progress-step {
  align-items: center;
  color: #7c8ca5;
  display: inline-flex;
  font-size: 16px;
  gap: 10px;
}

.progress-step strong {
  font-weight: 700;
}

.step-number {
  align-items: center;
  border: 1px solid #c8d5e5;
  border-radius: 50%;
  display: inline-flex;
  font-size: 16px;
  font-weight: 750;
  height: 38px;
  justify-content: center;
  width: 38px;
}

.progress-step.active,
.progress-step.completed {
  color: #0891b2;
}

.progress-step.active .step-number,
.progress-step.completed .step-number {
  background: #0891b2;
  border-color: #0891b2;
  box-shadow: 0 5px 13px rgba(8, 145, 178, 0.24);
  color: #fff;
}

.progress-line {
  background: #9dbbd1;
  height: 1px;
  margin: 0 34px;
  width: 48px;
}

.binding-header {
  margin-bottom: 30px;
}

.eyebrow {
  color: #0f766e;
  font-size: 14px;
  font-weight: 750;
  letter-spacing: 0.03em;
  margin: 0 0 10px;
}

h1 {
  color: #12213b;
  font-size: 30px;
  letter-spacing: -0.02em;
  line-height: 1.22;
  margin: 0;
}

.description {
  color: #536b8b;
  font-size: 16px;
  line-height: 1.75;
  margin: 13px 0 0;
  max-width: 620px;
}

.account-comparison {
  align-items: center;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 146px minmax(0, 1fr);
  margin-top: 8px;
}

.identity-card {
  background: #fff;
  border: 1px solid #d7e1ed;
  border-radius: 12px;
  box-sizing: border-box;
  min-height: 176px;
  padding: 24px 20px;
}

.identity-label {
  background: #edf7ff;
  border-radius: 5px;
  color: #1677d2;
  display: inline-flex;
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
  padding: 6px 9px;
}

.identity-content {
  align-items: center;
  display: flex;
  gap: 16px;
  margin-top: 18px;
  min-width: 0;
}

.avatar {
  align-items: center;
  background: #dce7f2;
  border-radius: 50%;
  color: #3d5876;
  display: flex;
  flex: 0 0 62px;
  font-size: 23px;
  font-weight: 750;
  height: 62px;
  justify-content: center;
  overflow: hidden;
  width: 62px;
}

.avatar img {
  height: 100%;
  object-fit: cover;
  width: 100%;
}

.provider-avatar {
  background: #f3f6fa;
  color: #172033;
}

.identity-copy {
  display: grid;
  gap: 7px;
  min-width: 0;
}

.identity-copy strong {
  align-items: center;
  color: #101d35;
  display: inline-flex;
  font-size: 19px;
  gap: 8px;
  overflow-wrap: anywhere;
}

.identity-copy span {
  color: #58708f;
  font-size: 15px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.binding-connector {
  align-items: center;
  color: #4b6687;
  display: flex;
}

.binding-connector > span {
  border-top: 1px dashed #b7c8db;
  flex: 1;
}

.binding-connector > div {
  align-items: center;
  background: #fff;
  border: 1px solid #cbd8e6;
  border-radius: 50%;
  display: flex;
  height: 56px;
  justify-content: center;
  width: 56px;
}

.binding-summary {
  align-items: center;
  color: #68809f;
  display: flex;
  font-size: 15px;
  gap: 9px;
  justify-content: center;
  margin: 24px 0;
  text-align: center;
}

.notice {
  align-items: flex-start;
  background: #eff7ff;
  border: 1px solid #d4e8ff;
  border-radius: 9px;
  color: #39709f;
  display: flex;
  font-size: 15px;
  gap: 13px;
  line-height: 1.65;
  padding: 16px 18px;
}

.notice svg {
  color: #1677d2;
  flex: 0 0 auto;
  margin-top: 2px;
}

.notice p {
  margin: 0;
}

.actions {
  display: flex;
  gap: 14px;
  justify-content: flex-end;
  margin-top: 38px;
}

.secondary-button,
.primary-button {
  border: 1px solid transparent;
  font-size: 16px;
  min-height: 48px;
  padding: 0 26px;
}

.secondary-button {
  background: #fff;
  border-color: #c7d4e3;
  color: #263c5c;
}

.primary-button {
  background: linear-gradient(90deg, #0a9ec0, #0784a6);
  box-shadow: 0 7px 14px rgba(8, 145, 178, 0.18);
  color: #fff;
}

.primary-button:hover:not(:disabled) {
  background: #087b99;
}

button:disabled {
  cursor: not-allowed;
  opacity: 0.56;
}

.state-panel {
  align-items: center;
  color: #475569;
  display: flex;
  flex-direction: column;
  min-height: 460px;
  justify-content: center;
  text-align: center;
}

.error-panel h1 {
  color: #b91c1c;
  font-size: 25px;
}

.error-panel p {
  line-height: 1.6;
  margin: 12px 0 0;
}

.error-actions {
  margin-top: 24px;
}

.loading-spinner {
  animation: spin 0.8s linear infinite;
  border: 3px solid #bae6fd;
  border-radius: 50%;
  border-top-color: #0891b2;
  height: 28px;
  width: 28px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 760px) {
  .binding-page {
    padding: 16px;
  }

  .binding-card {
    min-height: 0;
    padding: 24px;
  }

  .progress {
    justify-content: flex-start;
    margin-bottom: 30px;
  }

  .progress-step {
    font-size: 14px;
    gap: 7px;
  }

  .step-number {
    height: 32px;
    width: 32px;
  }

  .progress-line {
    margin: 0 14px;
    width: 22px;
  }

  h1 {
    font-size: 26px;
  }

  .account-comparison {
    gap: 14px;
    grid-template-columns: 1fr;
  }

  .binding-connector {
    display: none;
  }

  .actions {
    flex-direction: column-reverse;
    margin-top: 28px;
  }

  .actions button {
    width: 100%;
  }
}
</style>

<template>
  <AuthLayout class="login-page" :title="`登录 ${targetName}`" :target-name="targetName" :logo="targetLogo" show-logo show-docs-link fixed-mobile>
    <template #corner>
      <button type="button" class="login-mode-switch" :disabled="busy || qrCompleting" :aria-pressed="qrMode"
        :aria-label="qrMode ? '返回账号登录' : '切换到扫码登录'" :title="qrMode ? '账号登录' : '扫码登录'" @click="qrMode = !qrMode">
        <Monitor v-if="qrMode" /><QrCode v-else />
      </button>
    </template>

    <div v-if="errorMessage" class="auth-notice" role="alert">{{ errorMessage }}</div>
    <div class="login-method">
      <QRLoginPanel v-if="qrMode" :redirect="redirectUrl" @complete="finishLogin" @busy="qrCompleting = $event" />
      <template v-else>
        <div class="login-tabs" role="tablist" aria-label="登录方式">
          <button v-for="tab in tabs" :id="`tab-${tab.key}`" :key="tab.key" type="button" role="tab"
            :aria-selected="activeTab === tab.key" aria-controls="login-form" :tabindex="activeTab === tab.key ? 0 : -1"
            :disabled="busy" :class="{ active: activeTab === tab.key }" @click="activeTab = tab.key" @keydown="switchTab($event)">
            {{ tab.label }}
          </button>
        </div>
        <el-form id="login-form" ref="formRef" :model="form" :rules="rules" role="tabpanel"
          :aria-labelledby="`tab-${activeTab}`" @submit.prevent="handleLogin">
          <el-form-item prop="email">
            <el-input id="login-email" v-model="form.email" type="email" autocomplete="username" aria-label="邮箱"
              placeholder="请输入邮箱" :disabled="busy" />
          </el-form-item>
          <template v-if="activeTab === 'password'">
            <el-form-item prop="password">
              <el-input id="login-password" v-model="form.password" :type="showPassword ? 'text' : 'password'"
                autocomplete="current-password" aria-label="密码" placeholder="请输入密码" :disabled="busy">
                <template #suffix>
                  <button type="button" class="auth-icon-button" :disabled="busy" :aria-label="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword">
                    <EyeOff v-if="showPassword" /><Eye v-else />
                  </button>
                </template>
              </el-input>
            </el-form-item>
            <div v-if="captchaRequired" class="login-captcha">
              <label class="auth-label" for="login-captcha">图形验证码</label>
              <div class="auth-captcha-row">
                <el-input id="login-captcha" v-model="captchaInput" maxlength="4" inputmode="numeric" placeholder="请输入验证码" :disabled="busy" />
                <button type="button" class="auth-captcha-refresh" :disabled="busy" aria-label="刷新图形验证码" @click="loadCaptcha">
                  <img v-if="captchaImage" :src="captchaImage" alt="图形验证码" /><span v-else>点击重试</span>
                </button>
              </div>
            </div>
          </template>
          <template v-else>
            <el-form-item prop="code">
              <el-input id="login-code" v-model="code" maxlength="6" inputmode="numeric" autocomplete="one-time-code"
                aria-label="邮箱验证码" placeholder="6 位验证码" :disabled="busy">
                <template #suffix>
                  <button type="button" class="auth-code-button" :disabled="busy || countdown > 0" @click="openCodeModal">
                    {{ countdown > 0 ? `${countdown}秒后重发` : '获取验证码' }}
                  </button>
                </template>
              </el-input>
            </el-form-item>
          </template>
          <div class="login-options">
            <RememberLoginOption v-model="rememberMe" :disabled="busy" />
            <router-link v-if="activeTab === 'password'" :to="resetRoute">忘记密码？</router-link>
          </div>
          <button type="submit" class="auth-primary" :disabled="busy || (activeTab === 'password' && lockSeconds > 0)">
            <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />
            {{ busy ? '登录中…' : activeTab === 'password' && lockSeconds > 0 ? `${lockSeconds} 秒后重试` : '登录' }}
          </button>
        </el-form>
      </template>
    </div>
    <template #footer>
      <p v-if="!qrMode" class="auth-switch-link">还没有账号？<router-link :to="registerRoute">立即注册</router-link></p>
      <AuthProviders :disabled="busy || qrCompleting" @select="oauthLogin" />
    </template>
  </AuthLayout>
  <SendCodeModal :visible="codeModalVisible" :email="form.email" purpose="LOGIN" @close="codeModalVisible = false" @success="acceptCode" />
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Eye, EyeOff, LoaderCircle, Monitor, QrCode } from 'lucide-vue-next'
import { authAPI, setAccessToken } from '../api/auth'
import AuthLayout from '../components/AuthLayout.vue'
import AuthProviders from '../components/AuthProviders.vue'
import QRLoginPanel from '../components/QRLoginPanel.vue'
import RememberLoginOption from '../components/RememberLoginOption.vue'
import SendCodeModal from '../components/SendCodeModal.vue'
import { getLoginRedirect, loadTargetClient } from '../utils/oauthTarget'
import { normalizeAuthEmail, useAuthSubmission, useDeadline, useEmailChallenge, validAuthEmail } from '../utils/authState'

const route = useRoute()
const redirectUrl = getLoginRedirect(route)
const targetClient = ref(null)
const targetName = computed(() => targetClient.value?.name || '账号中心')
const targetLogo = computed(() => targetClient.value?.logo_url || (targetClient.value ? '' : '/assets/sso.svg'))
const routeFor = path => ({ path, query: redirectUrl === '/profile' ? {} : { redirect: redirectUrl } })
const registerRoute = routeFor('/register')
const resetRoute = routeFor('/reset-password')
const tabs = [{ key: 'password', label: '密码登录' }, { key: 'email', label: '验证码登录' }]
const activeTab = ref('password')
const qrMode = ref(false)
const qrCompleting = ref(false)
const rememberMe = ref(true)
const showPassword = ref(false)
const errorMessage = ref('')
const formRef = ref(null)
const form = reactive({ email: '', password: '' })
const { busy, run } = useAuthSubmission()
const { seconds: lockSeconds, start: lock, stop: clearLock } = useDeadline()
const { code, challengeId, valid: challengeValid, countdown, visible: codeModalVisible, accept: acceptCode } = useEmailChallenge(() => form.email)
const captchaRequired = ref(false)
const captchaInput = ref('')
const captchaID = ref('')
const captchaImage = ref('')
let captchaGeneration = 0
const rules = computed(() => ({
  email: [{ validator: (_rule, value, callback) => callback(validAuthEmail(value) ? undefined : new Error('请输入正确的邮箱地址')), trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
  code: [{ validator: (_rule, _value, callback) => callback(/^\d{6}$/.test(code.value) ? undefined : new Error('请输入六位验证码')), trigger: 'blur' }]
}))
const switchTab = event => {
  if (busy.value || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  activeTab.value = event.key === 'Home' ? 'password' : event.key === 'End' ? 'email' : activeTab.value === 'password' ? 'email' : 'password'
  document.getElementById(`tab-${activeTab.value}`)?.focus()
}
const loadCaptcha = async () => {
  const version = ++captchaGeneration
  captchaInput.value = captchaID.value = captchaImage.value = ''
  try {
    const response = await authAPI.getCaptcha()
    if (version !== captchaGeneration) return
    const data = response.data || response
    captchaID.value = data.captcha_id || ''
    captchaImage.value = data.captcha_png_base64 || ''
  } catch (error) {
    if (version === captchaGeneration) errorMessage.value = error.message || '获取图形验证码失败'
  }
}
const finishLogin = response => {
  busy.value = true
  const data = response.data || response
  setAccessToken(data.access_token)
  window.location.assign(data.redirect_url || redirectUrl)
}
const openCodeModal = () => {
  if (busy.value || countdown.value > 0) return
  if (!validAuthEmail(form.email)) { errorMessage.value = '请输入正确的邮箱地址'; return }
  errorMessage.value = ''
  codeModalVisible.value = true
}
const handleLogin = () => run(async isCurrent => {
  if (qrMode.value || (activeTab.value === 'password' && lockSeconds.value > 0)) return
  errorMessage.value = ''
  const mode = activeTab.value
  try { await formRef.value.validate() } catch { return }
  if (!isCurrent()) return
  if (mode === 'email' && !challengeValid.value) { errorMessage.value = '请先获取有效的邮箱验证码'; return }
  if (mode === 'password' && captchaRequired.value && (!captchaID.value || !/^\d{4}$/.test(captchaInput.value))) {
    errorMessage.value = '请完成图形验证码后再登录'; return
  }
  try {
    const response = mode === 'password'
      ? await authAPI.loginWithPassword({ email: normalizeAuthEmail(form.email), password: form.password, redirect: redirectUrl, remember_me: rememberMe.value,
        ...(captchaRequired.value ? { captcha_id: captchaID.value, captcha: captchaInput.value } : {}) })
      : await authAPI.loginWithEmail({ challenge_id: challengeId.value, code: code.value, redirect: redirectUrl, remember_me: rememberMe.value })
    if (!isCurrent()) return
    finishLogin(response)
    return true
  } catch (error) {
    if (!isCurrent()) return
    if (mode === 'password' && error.data?.code === 'CAPTCHA_REQUIRED') {
      captchaRequired.value = true
      errorMessage.value = '请完成图形验证码后再登录'
      await loadCaptcha()
    } else if (mode === 'password' && error.status === 429) {
      lock(Math.max(1, Number(error.data?.retry_after_seconds) || 1))
      form.password = ''
      errorMessage.value = error.message || '请求过于频繁，请稍后重试'
    } else {
      errorMessage.value = error.message || '登录失败'
      if (mode === 'password' && captchaRequired.value) await loadCaptcha()
    }
  }
})
const oauthLogin = provider => {
  if (busy.value || qrCompleting.value) return
  busy.value = true
  window.location.assign(`/api/auth/third/${provider}?redirect=${encodeURIComponent(redirectUrl)}&remember_me=true`)
}
watch(() => normalizeAuthEmail(form.email), () => {
  clearLock()
  captchaGeneration++
  captchaRequired.value = false
  captchaInput.value = captchaID.value = captchaImage.value = ''
  errorMessage.value = ''
})
watch([activeTab, qrMode], () => { errorMessage.value = ''; formRef.value?.clearValidate() })
onMounted(() => {
  void loadTargetClient(redirectUrl).then(client => { targetClient.value = client })
})
onUnmounted(() => {
  captchaGeneration++
})
</script>

<style scoped>
.login-method { --login-method-height: 304px; --qr-registration-space: 44px; min-height: var(--login-method-height); }
.login-tabs { display: flex; gap: 28px; height: 45px; margin-bottom: 18px; border-bottom: 1px solid #e9e9e9; }
.login-tabs button { position: relative; color: #999; font-size: 15px; padding-bottom: 12px; }
.login-tabs button.active { color: #171717; font-weight: 600; }
.login-tabs button.active::after { content: ''; position: absolute; height: 2px; background: #171717; left: 0; right: 0; bottom: -1px; }
.login-mode-switch { position: absolute; top: 0; right: 0; width: 62px; height: 62px; border-top-right-radius: 16px; }
.login-mode-switch::before { content: ''; position: absolute; inset: 0; border-radius: inherit; background: #f5f5f5; clip-path: polygon(0 0, 100% 0, 100% 100%); }
.login-mode-switch svg { position: absolute; top: 9px; right: 9px; width: 23px; height: 23px; }
.login-mode-switch:hover::before { background: #e9e9e9; }
.login-mode-switch:focus-visible { outline-offset: -3px; }
.login-options { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 20px; margin-bottom: 14px; font-size: 12px; }
.login-captcha { margin-bottom: 20px; }
@media (max-height: 700px) { .login-method { --qr-registration-space: 40px; } }
@media (max-width: 767px) {
  .login-method { --login-method-height: 288px; --qr-registration-space: 0px; min-height: 0; }
  .login-mode-switch { top: 0; right: 0; border-radius: 0; }
}
</style>

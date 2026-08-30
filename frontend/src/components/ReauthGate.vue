<template>
  <Teleport to="body">
    <main v-if="reauthState.visible" class="reauth-page" role="dialog" aria-modal="true" aria-labelledby="reauth-title">
      <section class="reauth-shell">
        <h1 id="reauth-title">Confirm access</h1>

        <section class="account-card" aria-label="当前登录账号">
          <img v-if="avatarURL" class="account-avatar" :src="avatarURL" alt="" />
          <span v-else class="account-avatar account-avatar-fallback" aria-hidden="true">{{ avatarInitial }}</span>
          <p>已登录为 <strong>{{ accountLabel }}</strong></p>
        </section>

        <section class="verification-card passkey-card" :class="{ 'is-unavailable': !canUsePasskey }">
          <span class="verification-icon" aria-hidden="true"><Fingerprint :size="29" /></span>
          <h2>Passkey</h2>

          <template v-if="canUsePasskey">
            <p>准备好后，使用当前设备、安全密钥或其他设备上的 Passkey 完成验证。</p>
            <p v-if="activeMethod === 'passkey' && errorMessage" class="error-message" role="alert">{{ errorMessage }}</p>
            <button class="primary-button" type="button" :disabled="submitting || enrollmentSubmitting" @click="verifyPasskey">
              {{ submitting && activeMethod === 'passkey' ? '验证中…' : '使用 Passkey' }}
            </button>
          </template>

          <template v-else-if="!hasEmail">
            <p>当前账号没有邮箱或 Passkey。请重新登录完成身份确认，再继续当前操作。</p>
            <button class="secondary-action-button" type="button" :disabled="submitting || enrollmentSubmitting" @click="reauthenticate">
              重新登录
            </button>
          </template>

          <template v-else-if="activeMethod === 'setup'">
            <PasskeyEnrollmentForm @registered="handlePasskeyRegistered" @submitting="enrollmentSubmitting = $event" />
          </template>

          <template v-else>
            <p>尚未设置 Passkey。设置后可使用设备生物识别、锁屏密码或安全密钥完成验证。</p>
            <button class="primary-button" type="button" :disabled="submitting || enrollmentSubmitting" @click="startPasskeyEnrollment">
              开始设置 Passkey
            </button>
          </template>
        </section>

        <section class="verification-card alternatives-card">
          <h2>其他验证方式</h2>
          <p class="alternatives-description">无法使用 Passkey？请选择以下方式完成验证。</p>

          <template v-if="hasEmail">
            <button class="email-method" type="button" :disabled="submitting || enrollmentSubmitting" @click="selectEmail">
              <span class="email-method-icon" aria-hidden="true"><Mail :size="18" /></span>
              <span class="email-method-copy">
                <strong>邮箱验证码</strong>
                <small>发送验证码至 {{ emailHint }}</small>
              </span>
              <span class="email-method-action">{{ activeMethod === 'email' ? '已选择' : '使用' }}</span>
            </button>

            <div v-if="activeMethod === 'email'" class="email-form">
              <p v-if="errorMessage" class="error-message" role="alert">{{ errorMessage }}</p>

              <template v-if="emailStep === 'captcha'">
                <label class="field-label" for="reauth-captcha">图形验证码</label>
                <div class="captcha-row">
                  <input id="reauth-captcha" v-model="captcha" maxlength="4" autocomplete="off" placeholder="输入四位验证码" @keyup.enter="sendEmailCode" />
                  <button class="captcha-button" type="button" title="刷新图形验证码" :disabled="submitting" @click="loadCaptcha">
                    <img v-if="captchaImage" :src="captchaImage" alt="图形验证码" />
                    <span v-else>加载中</span>
                  </button>
                </div>
                <button class="primary-button" type="button" :disabled="submitting || captcha.length !== 4" @click="sendEmailCode">
                  {{ submitting ? '发送中…' : '发送验证码' }}
                </button>
              </template>

              <template v-else>
                <label class="field-label" for="reauth-email-code">邮箱验证码</label>
                <input id="reauth-email-code" v-model="emailCode" class="code-input" maxlength="6" inputmode="numeric" autocomplete="one-time-code" placeholder="输入六位验证码" @keyup.enter="verifyEmailCode" />
                <button class="primary-button" type="button" :disabled="submitting || emailCode.length !== 6" @click="verifyEmailCode">
                  {{ submitting ? '验证中…' : '确认访问' }}
                </button>
                <button class="link-button" type="button" :disabled="submitting || resendSeconds > 0" @click="restartEmail">
                  {{ resendSeconds > 0 ? `${resendSeconds} 秒后可重新发送` : '重新发送验证码' }}
                </button>
              </template>
            </div>
          </template>

          <p v-else class="empty-methods">重新登录后，可在短时间内继续当前敏感操作。</p>
        </section>

        <footer class="reauth-footer">
          <button class="cancel-button" type="button" :disabled="submitting || enrollmentSubmitting" @click="cancelReauth">取消并返回</button>
          <p class="reauth-tip"><ShieldCheck :size="15" /> 提示：验证成功后，当前操作会自动继续。</p>
        </footer>
      </section>
    </main>
  </Teleport>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { startAuthentication } from '@simplewebauthn/browser'
import { Fingerprint, Mail, ShieldCheck } from 'lucide-vue-next'
import { authAPI, reauthAPI } from '../api/auth'
import PasskeyEnrollmentForm from './PasskeyEnrollmentForm.vue'
import { cancelReauth, completeReauth, reauthState } from '../utils/reauthCoordinator'

const router = useRouter()
const activeMethod = ref('')
const emailStep = ref('captcha')
const captcha = ref('')
const captchaID = ref('')
const captchaImage = ref('')
const challengeID = ref('')
const emailCode = ref('')
const submitting = ref(false)
const enrollmentSubmitting = ref(false)
const passkeyRegistered = ref(false)
const errorMessage = ref('')
const resendSeconds = ref(0)
let resendTimer = null

const methods = computed(() => reauthState.descriptor?.methods || [])
const hasPasskey = computed(() => methods.value.includes('passkey'))
const hasEmail = computed(() => methods.value.includes('email'))
const canUsePasskey = computed(() => hasPasskey.value || passkeyRegistered.value)
const emailHint = computed(() => reauthState.descriptor?.email_hint || '已绑定邮箱')
const accountLabel = computed(() => reauthState.descriptor?.username || reauthState.descriptor?.email_hint || '当前账号')
const avatarURL = computed(() => reauthState.descriptor?.avatar_url || '')
const avatarInitial = computed(() => accountLabel.value.slice(0, 1).toUpperCase())

const reset = () => {
  stopResendTimer()
  activeMethod.value = hasPasskey.value ? 'passkey' : (hasEmail.value ? '' : 'unavailable')
  emailStep.value = 'captcha'
  captcha.value = ''
  captchaID.value = ''
  captchaImage.value = ''
  challengeID.value = ''
  emailCode.value = ''
  submitting.value = false
  enrollmentSubmitting.value = false
  passkeyRegistered.value = false
  errorMessage.value = ''
  resendSeconds.value = 0
}

const verifyPasskey = async () => {
  if (!canUsePasskey.value || submitting.value || enrollmentSubmitting.value) return
  activeMethod.value = 'passkey'
  if (!window.PublicKeyCredential) {
    errorMessage.value = '当前浏览器不支持 Passkey，请改用邮箱验证码。'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    const optionsResult = await reauthAPI.passkeyOptions()
    const payload = optionsResult?.data || optionsResult
    const response = await startAuthentication({ optionsJSON: payload.options?.publicKey || payload.options })
    const verifyResult = await reauthAPI.passkeyVerify({ ceremony_id: payload.ceremony_id, response })
    completeReauth(verifyResult?.data || verifyResult)
  } catch (error) {
    if (error?.name === 'NotAllowedError' || error?.name === 'AbortError') {
      errorMessage.value = 'Passkey 验证已取消，可以重试或改用邮箱验证码。'
    } else {
      errorMessage.value = error.message || 'Passkey 验证失败，请重试。'
    }
  } finally {
    submitting.value = false
  }
}

const startPasskeyEnrollment = () => {
  if (submitting.value || enrollmentSubmitting.value || !hasEmail.value) return
  activeMethod.value = 'setup'
  errorMessage.value = ''
}

const handlePasskeyRegistered = async () => {
  passkeyRegistered.value = true
  activeMethod.value = 'passkey'
  await verifyPasskey()
}

const selectEmail = () => {
  if (submitting.value || enrollmentSubmitting.value || activeMethod.value === 'email') return
  activeMethod.value = 'email'
  restartEmail()
}

const reauthenticate = () => {
  if (submitting.value || enrollmentSubmitting.value) return
  cancelReauth()
  window.location.href = `/login?redirect=${encodeURIComponent(router.currentRoute.value.fullPath)}`
}

const loadCaptcha = async () => {
  errorMessage.value = ''
  try {
    const result = await authAPI.getCaptcha()
    const data = result?.data || result
    captchaID.value = data.captcha_id
    captchaImage.value = data.captcha_png_base64
    captcha.value = ''
  } catch (error) {
    errorMessage.value = error.message || '无法加载图形验证码。'
  }
}

const sendEmailCode = async () => {
  if (captcha.value.length !== 4 || submitting.value) return
  submitting.value = true
  errorMessage.value = ''
  try {
    const result = await reauthAPI.sendEmail({ captcha_id: captchaID.value, captcha: captcha.value })
    const data = result?.data || result
    challengeID.value = data.challenge_id
    emailStep.value = 'code'
    startResendTimer(Number(data.resend_after || 60))
  } catch (error) {
    const message = error.message || '验证码发送失败。'
    await loadCaptcha()
    errorMessage.value = message
  } finally {
    submitting.value = false
  }
}

const verifyEmailCode = async () => {
  if (emailCode.value.length !== 6 || submitting.value) return
  submitting.value = true
  errorMessage.value = ''
  try {
    const result = await reauthAPI.verifyEmail({ challenge_id: challengeID.value, code: emailCode.value })
    completeReauth(result?.data || result)
  } catch (error) {
    errorMessage.value = error.message || '邮箱验证失败，请重试。'
  } finally {
    submitting.value = false
  }
}

const restartEmail = () => {
  stopResendTimer()
  emailStep.value = 'captcha'
  challengeID.value = ''
  emailCode.value = ''
  errorMessage.value = ''
  resendSeconds.value = 0
  loadCaptcha()
}

const startResendTimer = (seconds) => {
  stopResendTimer()
  resendSeconds.value = Math.max(0, seconds)
  resendTimer = window.setInterval(() => {
    resendSeconds.value = Math.max(0, resendSeconds.value - 1)
    if (resendSeconds.value === 0) stopResendTimer()
  }, 1000)
}

const stopResendTimer = () => {
  if (resendTimer) window.clearInterval(resendTimer)
  resendTimer = null
}

watch(() => reauthState.requestID, reset)
onBeforeUnmount(stopResendTimer)
</script>

<style scoped>
.reauth-page { position: fixed; inset: 0; z-index: 200; box-sizing: border-box; display: flex; justify-content: center; overflow-y: auto; padding: 48px 24px 40px; background: var(--profile-surface); color: var(--profile-text); }
.reauth-shell { width: min(320px, 100%); }
h1 { margin: 0 0 20px; font-size: 24px; font-weight: 500; letter-spacing: -.02em; text-align: center; }
.account-card, .verification-card { box-sizing: border-box; border: 1px solid var(--profile-border); border-radius: 6px; background: var(--profile-surface); box-shadow: var(--profile-button-shadow); }
.account-card { display: flex; align-items: center; gap: 13px; min-height: 74px; padding: 14px 17px; }
.account-card p { margin: 0; color: var(--profile-text-muted); font-size: 14px; }
.account-card strong { color: var(--profile-text); font-weight: 600; }
.account-avatar { display: block; flex: 0 0 38px; width: 38px; height: 38px; border-radius: 50%; object-fit: cover; }
.account-avatar-fallback { display: grid; place-items: center; background: var(--profile-accent-soft); color: var(--profile-accent); font-size: 16px; font-weight: 600; }
.verification-card { margin-top: 16px; padding: 20px 17px 18px; }
.passkey-card { background: var(--profile-surface-subtle); text-align: center; }
.verification-icon { display: grid; width: 46px; height: 46px; margin: 0 auto 10px; place-items: center; color: var(--profile-text-muted); }
h2 { margin: 0; font-size: 18px; font-weight: 600; }
.passkey-card > p:not(.error-message) { margin: 17px 0 15px; color: var(--profile-text-strong); font-size: 14px; line-height: 1.55; text-align: left; }
.is-unavailable { background: var(--profile-surface-subtle); }
.is-unavailable .verification-icon { color: var(--profile-text-faint); }
.primary-button, .secondary-action-button, .cancel-button, .link-button, .captcha-button, .email-method { font: inherit; cursor: pointer; }
.primary-button { width: 100%; min-height: 36px; border: 1px solid var(--profile-border-muted); border-radius: 6px; background: var(--profile-success-strong); box-shadow: var(--profile-button-shadow); color: #fff; font-size: 14px; font-weight: 600; }
.primary-button:hover:not(:disabled) { background: var(--profile-success); }
.secondary-action-button { width: 100%; min-height: 36px; border: 1px solid var(--profile-border-muted); border-radius: 6px; background: var(--profile-surface); color: var(--profile-text-strong); font-size: 14px; font-weight: 600; }
.secondary-action-button:hover:not(:disabled) { background: var(--profile-surface-subtle); }
.alternatives-card { padding: 18px 17px; }
.alternatives-card h2 { font-size: 16px; }
.alternatives-description { margin: 8px 0 13px; color: var(--profile-text-muted); font-size: 13px; line-height: 1.5; }
.email-method { display: flex; width: 100%; align-items: center; gap: 10px; padding: 10px 0; border: 0; border-top: 1px solid var(--profile-divider); background: transparent; color: var(--profile-accent); text-align: left; }
.email-method:hover:not(:disabled) .email-method-copy strong { text-decoration: underline; }
.email-method-icon { display: grid; flex: 0 0 30px; height: 30px; place-items: center; border-radius: 6px; background: var(--profile-accent-soft); color: var(--profile-accent); }
.email-method-copy { display: grid; min-width: 0; gap: 2px; }
.email-method-copy strong { color: var(--profile-accent); font-size: 14px; font-weight: 600; }
.email-method-copy small { overflow: hidden; color: var(--profile-text-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.email-method-action { margin-left: auto; color: var(--profile-text-muted); font-size: 12px; }
.email-form { margin-top: 6px; padding-top: 14px; border-top: 1px solid var(--profile-divider); }
.field-label { display: block; color: var(--profile-text-strong); font-size: 13px; font-weight: 600; text-align: left; }
input { box-sizing: border-box; min-width: 0; height: 34px; border: 1px solid var(--profile-border); border-radius: 6px; padding: 5px 8px; outline: none; background: var(--profile-surface); color: var(--profile-text); font: inherit; font-size: 14px; }
input:focus { border-color: var(--profile-accent); box-shadow: 0 0 0 3px var(--profile-focus-ring); }
.captcha-row { display: grid; grid-template-columns: minmax(0, 1fr) 100px; gap: 8px; margin-top: 7px; }
.captcha-button { display: grid; height: 34px; overflow: hidden; place-items: center; border: 1px solid var(--profile-border); border-radius: 6px; background: var(--profile-surface-subtle); color: var(--profile-text-muted); font-size: 12px; }
.captcha-button img { width: 100%; height: 100%; object-fit: contain; }
.email-form .primary-button { margin-top: 12px; }
.code-input { width: 100%; margin-top: 7px; text-align: center; letter-spacing: .22em; }
.link-button { display: block; width: 100%; margin-top: 12px; border: 0; background: transparent; color: var(--profile-accent); font-size: 13px; }
.error-message { margin: 0 0 12px; padding: 8px 10px; border: 1px solid var(--profile-danger-border); border-radius: 6px; background: var(--profile-danger-soft); color: var(--profile-danger); font-size: 13px; line-height: 1.45; text-align: left; }
.empty-methods { margin: 0; color: var(--profile-text-faint); font-size: 13px; }
.reauth-footer { margin-top: 16px; text-align: center; }
.cancel-button { border: 0; background: transparent; color: var(--profile-accent); font-size: 13px; }
.reauth-tip { display: flex; align-items: flex-start; justify-content: center; gap: 5px; margin: 13px 0 0; color: var(--profile-text-faint); font-size: 12px; line-height: 1.5; }
.reauth-tip svg { flex: 0 0 auto; margin-top: 2px; }
button:disabled { cursor: not-allowed; opacity: .55; }
@media (max-width: 520px) { .reauth-page { padding: 32px 16px 26px; } }
</style>

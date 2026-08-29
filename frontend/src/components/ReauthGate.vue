<template>
  <Teleport to="body">
    <main v-if="reauthState.visible" class="reauth-page" role="dialog" aria-modal="true" aria-labelledby="reauth-title">
      <section class="reauth-shell">
        <h1 id="reauth-title">Confirm access</h1>

        <div class="account-card">
          <span class="account-avatar" aria-hidden="true"><UserRound :size="20" /></span>
          <span>已登录为 <strong>{{ accountLabel }}</strong></span>
        </div>

        <section v-if="activeMethod === 'passkey'" class="method-card">
          <span class="method-icon" aria-hidden="true"><Fingerprint :size="31" /></span>
          <h2>Passkey</h2>
          <p>准备好后，使用当前设备、安全密钥或其他设备上的 Passkey 完成验证。</p>
          <p v-if="errorMessage" class="error-message" role="alert">{{ errorMessage }}</p>
          <button class="primary-button" type="button" :disabled="submitting" @click="verifyPasskey">
            {{ submitting ? '验证中…' : 'Use Passkey' }}
          </button>
          <button v-if="hasEmail" class="link-button" type="button" :disabled="submitting" @click="selectEmail">
            改用邮箱验证码
          </button>
        </section>

        <section v-else-if="activeMethod === 'email'" class="method-card email-card">
          <span class="method-icon" aria-hidden="true"><Mail :size="29" /></span>
          <h2>邮箱验证码</h2>
          <p>验证码将发送到当前账号邮箱 {{ emailHint }}。</p>
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

          <button v-if="hasPasskey" class="link-button secondary-link" type="button" :disabled="submitting" @click="selectPasskey">
            返回使用 Passkey
          </button>
        </section>

        <section v-else class="method-card unavailable-card">
          <span class="method-icon" aria-hidden="true"><ShieldAlert :size="29" /></span>
          <h2>无法确认访问</h2>
          <p>当前账号没有可用的 Passkey 或绑定邮箱，请先补充安全验证方式。</p>
        </section>

        <p class="reauth-tip"><ShieldCheck :size="15" /> 验证成功后，当前操作会自动继续。</p>
        <button class="cancel-button" type="button" :disabled="submitting" @click="cancelReauth">取消并返回</button>
      </section>
    </main>
  </Teleport>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { startAuthentication } from '@simplewebauthn/browser'
import { Fingerprint, Mail, ShieldAlert, ShieldCheck, UserRound } from 'lucide-vue-next'
import { authAPI, reauthAPI } from '../api/auth'
import { cancelReauth, completeReauth, reauthState } from '../utils/reauthCoordinator'

const activeMethod = ref('')
const emailStep = ref('captcha')
const captcha = ref('')
const captchaID = ref('')
const captchaImage = ref('')
const challengeID = ref('')
const emailCode = ref('')
const submitting = ref(false)
const errorMessage = ref('')
const resendSeconds = ref(0)
let resendTimer = null

const methods = computed(() => reauthState.descriptor?.methods || [])
const hasPasskey = computed(() => methods.value.includes('passkey'))
const hasEmail = computed(() => methods.value.includes('email'))
const emailHint = computed(() => reauthState.descriptor?.email_hint || '已绑定邮箱')
const accountLabel = computed(() => reauthState.descriptor?.email_hint || '当前账号')

const reset = () => {
  stopResendTimer()
  activeMethod.value = hasPasskey.value ? 'passkey' : (hasEmail.value ? 'email' : '')
  emailStep.value = 'captcha'
  captcha.value = ''
  captchaID.value = ''
  captchaImage.value = ''
  challengeID.value = ''
  emailCode.value = ''
  submitting.value = false
  errorMessage.value = ''
  resendSeconds.value = 0
  if (activeMethod.value === 'email') loadCaptcha()
}

const verifyPasskey = async () => {
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

const selectEmail = () => {
  activeMethod.value = 'email'
  errorMessage.value = ''
  restartEmail()
}

const selectPasskey = () => {
  stopResendTimer()
  activeMethod.value = 'passkey'
  errorMessage.value = ''
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
.reauth-page { position: fixed; inset: 0; z-index: 200; box-sizing: border-box; display: flex; justify-content: center; overflow-y: auto; padding: 72px 24px 40px; background: #f8fafc; color: #172033; }
.reauth-page::before { position: fixed; inset: 0; z-index: -1; background: radial-gradient(circle at 50% 0, rgba(8, 145, 178, .1), transparent 34%); content: ''; }
.reauth-shell { width: min(390px, 100%); text-align: center; }
h1 { margin: 0 0 24px; font-size: 28px; letter-spacing: -.03em; }
.account-card, .method-card { box-sizing: border-box; border: 1px solid #d8e1eb; border-radius: 12px; background: white; box-shadow: 0 8px 28px rgba(15, 23, 42, .05); }
.account-card { display: flex; align-items: center; gap: 12px; min-height: 72px; padding: 14px 18px; text-align: left; color: #475569; }
.account-avatar { display: grid; flex: 0 0 38px; height: 38px; place-items: center; border-radius: 50%; background: #ecfeff; color: #0e7490; }
.account-card strong { color: #172033; }
.method-card { display: flex; flex-direction: column; align-items: stretch; margin-top: 18px; padding: 26px 22px 22px; }
.method-icon { display: grid; width: 54px; height: 54px; margin: 0 auto 12px; place-items: center; border-radius: 15px; background: #ecfeff; color: #0e7490; }
h2 { margin: 0; font-size: 22px; }
.method-card > p:not(.error-message) { margin: 13px 0 20px; color: #64748b; font-size: 14px; line-height: 1.65; text-align: left; }
.primary-button, .cancel-button, .link-button, .captcha-button { font: inherit; cursor: pointer; }
.primary-button { width: 100%; min-height: 44px; margin-top: 14px; border: 0; border-radius: 9px; background: #0891b2; color: white; font-weight: 700; }
.primary-button:hover:not(:disabled) { background: #0e7490; }
.link-button { margin-top: 15px; border: 0; background: transparent; color: #087b9b; font-weight: 650; }
.secondary-link { padding-top: 14px; border-top: 1px solid #e2e8f0; }
.cancel-button { margin-top: 14px; border: 0; background: transparent; color: #64748b; font-weight: 600; }
button:disabled { cursor: not-allowed; opacity: .55; }
.error-message { margin: -3px 0 4px; padding: 10px 12px; border-radius: 8px; background: #fef2f2; color: #b91c1c; font-size: 13px; line-height: 1.45; text-align: left; }
.field-label { margin-top: 2px; color: #334155; font-size: 13px; font-weight: 650; text-align: left; }
input { box-sizing: border-box; min-width: 0; height: 44px; border: 1px solid #cbd5e1; border-radius: 9px; padding: 0 12px; outline: none; font: inherit; }
input:focus { border-color: #0891b2; box-shadow: 0 0 0 3px rgba(8, 145, 178, .13); }
.captcha-row { display: grid; grid-template-columns: minmax(0, 1fr) 112px; gap: 10px; margin-top: 7px; }
.captcha-button { display: grid; height: 44px; overflow: hidden; place-items: center; border: 1px solid #cbd5e1; border-radius: 9px; background: #f8fafc; color: #64748b; }
.captcha-button img { width: 100%; height: 100%; object-fit: contain; }
.code-input { width: 100%; margin-top: 7px; text-align: center; letter-spacing: .22em; }
.reauth-tip { display: flex; align-items: center; justify-content: center; gap: 6px; margin: 18px 0 0; color: #64748b; font-size: 13px; }
.unavailable-card p { text-align: center !important; }
@media (max-width: 520px) { .reauth-page { padding: 34px 16px 28px; } h1 { font-size: 25px; } .method-card { padding: 23px 18px 20px; } }
</style>

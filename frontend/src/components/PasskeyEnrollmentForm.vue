<template>
  <section class="passkey-enrollment-form">
    <p class="description">先验证当前账号邮箱，再使用设备生物识别、锁屏密码或安全密钥创建 Passkey。</p>
    <p v-if="errorMessage" class="error-message" role="alert">{{ errorMessage }}</p>

    <template v-if="step === 'captcha'">
      <label class="field-label" for="passkey-enrollment-captcha">图形验证码</label>
      <div class="captcha-row">
        <input id="passkey-enrollment-captcha" v-model="captcha" maxlength="4" autocomplete="off" placeholder="输入四位验证码" @keyup.enter="sendCode" />
        <button class="captcha-button" type="button" title="刷新图形验证码" :disabled="submitting" @click="loadCaptcha">
          <img v-if="captchaImage" :src="captchaImage" alt="图形验证码" />
          <span v-else>加载中</span>
        </button>
      </div>
      <button class="primary-button" type="button" :disabled="submitting || captcha.length !== 4" @click="sendCode">
        {{ submitting ? '发送中…' : '发送邮箱验证码' }}
      </button>
    </template>

    <template v-else>
      <label class="field-label" for="passkey-enrollment-code">邮箱验证码</label>
      <input id="passkey-enrollment-code" v-model="code" class="code-input" maxlength="6" inputmode="numeric" autocomplete="one-time-code" placeholder="输入六位验证码" @keyup.enter="register" />
      <button class="primary-button" type="button" :disabled="submitting || code.length !== 6" @click="register">
        {{ submitting ? '创建中…' : '创建 Passkey' }}
      </button>
      <button class="secondary-button" type="button" :disabled="submitting" @click="reset">重新发送验证码</button>
    </template>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { startRegistration } from '@simplewebauthn/browser'
import { authAPI, passkeyAPI } from '../api/auth'

const emit = defineEmits(['registered', 'submitting'])
const step = ref('captcha')
const captcha = ref('')
const captchaID = ref('')
const captchaImage = ref('')
const challengeID = ref('')
const code = ref('')
const submitting = ref(false)
const errorMessage = ref('')

const setSubmitting = (value) => {
  submitting.value = value
  emit('submitting', value)
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

const sendCode = async () => {
  if (captcha.value.length !== 4 || submitting.value) return
  setSubmitting(true)
  errorMessage.value = ''
  try {
    const result = await passkeyAPI.sendRegistrationEmail({ captcha_id: captchaID.value, captcha: captcha.value })
    const data = result?.data || result
    challengeID.value = data.challenge_id
    step.value = 'otp'
  } catch (error) {
    const message = error.message || '验证码发送失败。'
    await loadCaptcha()
    errorMessage.value = message
  } finally {
    setSubmitting(false)
  }
}

const register = async () => {
  if (code.value.length !== 6 || submitting.value) return
  if (!window.PublicKeyCredential) {
    errorMessage.value = '当前浏览器不支持 Passkey。'
    return
  }
  setSubmitting(true)
  errorMessage.value = ''
  let registered = false
  try {
    const optionsResult = await passkeyAPI.registrationOptions({ challenge_id: challengeID.value, code: code.value })
    const payload = optionsResult?.data || optionsResult
    const response = await startRegistration({ optionsJSON: payload.options?.publicKey || payload.options })
    await passkeyAPI.registrationVerify({ ceremony_id: payload.ceremony_id, response })
    registered = true
  } catch (error) {
    if (error?.name === 'NotAllowedError' || error?.name === 'AbortError') {
      errorMessage.value = 'Passkey 创建已取消，请重试。'
      return
    }
    errorMessage.value = error.message || 'Passkey 创建失败，请重试。'
  } finally {
    setSubmitting(false)
  }
  if (registered) emit('registered')
}

const reset = () => {
  step.value = 'captcha'
  challengeID.value = ''
  code.value = ''
  errorMessage.value = ''
  loadCaptcha()
}

onMounted(reset)
</script>

<style scoped>
.passkey-enrollment-form { margin-top: 14px; text-align: left; }
.description { margin: 0 0 14px; color: #57606a; font-size: 13px; line-height: 1.5; }
.error-message { margin: 0 0 12px; padding: 8px 10px; border: 1px solid #ff8182; border-radius: 6px; background: #ffebe9; color: #cf222e; font-size: 13px; line-height: 1.45; }
.field-label { display: block; color: #24292f; font-size: 13px; font-weight: 600; }
input { box-sizing: border-box; min-width: 0; height: 34px; border: 1px solid #d0d7de; border-radius: 6px; padding: 5px 8px; outline: none; background: #fff; color: #1f2328; font: inherit; font-size: 14px; }
input:focus { border-color: #0969da; box-shadow: 0 0 0 3px rgba(9, 105, 218, .3); }
.captcha-row { display: grid; grid-template-columns: minmax(0, 1fr) 100px; gap: 8px; margin-top: 7px; }
.captcha-button { display: grid; height: 34px; overflow: hidden; place-items: center; border: 1px solid #d0d7de; border-radius: 6px; background: #f6f8fa; color: #57606a; font: inherit; font-size: 12px; cursor: pointer; }
.captcha-button img { width: 100%; height: 100%; object-fit: contain; }
.primary-button, .secondary-button { width: 100%; min-height: 36px; border-radius: 6px; font: inherit; font-size: 14px; font-weight: 600; cursor: pointer; }
.primary-button { margin-top: 12px; border: 1px solid rgba(27, 31, 36, .15); background: #1f883d; box-shadow: 0 1px 0 rgba(31, 35, 40, .1); color: #fff; }
.primary-button:hover:not(:disabled) { background: #1a7f37; }
.secondary-button { margin-top: 10px; border: 0; background: transparent; color: #0969da; }
.code-input { width: 100%; margin-top: 7px; text-align: center; letter-spacing: .22em; }
button:disabled { cursor: not-allowed; opacity: .55; }
</style>

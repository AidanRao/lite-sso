<template>
  <Teleport to="body">
    <div v-if="visible" class="passkey-mask" @click.self="close">
      <section class="passkey-dialog" role="dialog" aria-modal="true" aria-labelledby="passkey-enrollment-title">
        <header>
          <div>
            <p>安全验证</p>
            <h2 id="passkey-enrollment-title">注册 Passkey</h2>
          </div>
          <button type="button" :disabled="submitting" aria-label="关闭" @click="close">×</button>
        </header>

        <p class="description">先验证当前账号邮箱，再使用设备生物识别、锁屏密码或安全密钥创建 Passkey。</p>
        <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>

        <template v-if="step === 'captcha'">
          <label>
            <span>图形验证码</span>
            <div class="captcha-row">
              <input v-model="captcha" maxlength="4" placeholder="输入四位验证码" @keyup.enter="sendCode" />
              <img :src="captchaImage" alt="图形验证码" title="点击刷新" @click="loadCaptcha" />
            </div>
          </label>
          <button class="primary" type="button" :disabled="submitting" @click="sendCode">{{ submitting ? '发送中…' : '发送邮箱验证码' }}</button>
        </template>

        <template v-else>
          <label>
            <span>邮箱验证码</span>
            <input v-model="code" maxlength="6" inputmode="numeric" placeholder="输入六位验证码" @keyup.enter="register" />
          </label>
          <button class="primary" type="button" :disabled="submitting" @click="register">{{ submitting ? '注册中…' : '创建 Passkey' }}</button>
          <button class="secondary" type="button" :disabled="submitting" @click="reset">重新发送验证码</button>
        </template>
      </section>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, watch } from 'vue'
import { startRegistration } from '@simplewebauthn/browser'
import { authAPI, passkeyAPI } from '../api/auth'

const props = defineProps({ visible: { type: Boolean, default: false } })
const emit = defineEmits(['close', 'registered'])
const step = ref('captcha')
const captcha = ref('')
const captchaID = ref('')
const captchaImage = ref('')
const challengeID = ref('')
const code = ref('')
const submitting = ref(false)
const errorMessage = ref('')

const loadCaptcha = async () => {
  try {
    const result = await authAPI.getCaptcha()
    const data = result?.data || result
    captchaID.value = data.captcha_id
    captchaImage.value = data.captcha_png_base64
    captcha.value = ''
  } catch (error) {
    errorMessage.value = error.message || '无法加载验证码'
  }
}

const sendCode = async () => {
  if (captcha.value.length !== 4) {
    errorMessage.value = '请输入四位图形验证码'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    const result = await passkeyAPI.sendRegistrationEmail({ captcha_id: captchaID.value, captcha: captcha.value })
    const data = result?.data || result
    challengeID.value = data.challenge_id
    step.value = 'otp'
  } catch (error) {
    errorMessage.value = error.message || '发送失败'
    await loadCaptcha()
  } finally {
    submitting.value = false
  }
}

const register = async () => {
  if (code.value.length !== 6) {
    errorMessage.value = '请输入六位邮箱验证码'
    return
  }
  if (!window.PublicKeyCredential) {
    errorMessage.value = '当前浏览器不支持 Passkey'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    const optionsResult = await passkeyAPI.registrationOptions({ challenge_id: challengeID.value, code: code.value })
    const payload = optionsResult?.data || optionsResult
    const response = await startRegistration({ optionsJSON: payload.options?.publicKey || payload.options })
    await passkeyAPI.registrationVerify({ ceremony_id: payload.ceremony_id, response })
    emit('registered')
    close()
  } catch (error) {
    if (error?.name === 'NotAllowedError' || error?.name === 'AbortError') {
      return
    }
    errorMessage.value = error.message || 'Passkey 注册失败'
  } finally {
    submitting.value = false
  }
}

const reset = () => {
  step.value = 'captcha'
  challengeID.value = ''
  code.value = ''
  errorMessage.value = ''
  loadCaptcha()
}

const close = () => {
  if (!submitting.value) emit('close')
}

watch(() => props.visible, (visible) => {
  if (visible) {
    reset()
  }
})
</script>

<style scoped>
.passkey-mask { position: fixed; inset: 0; z-index: 80; display: grid; place-items: center; padding: 20px; background: rgba(15, 23, 42, .55); backdrop-filter: blur(4px); }
.passkey-dialog { width: min(440px, 100%); box-sizing: border-box; padding: 26px; border-radius: 18px; background: white; box-shadow: 0 28px 70px rgba(15, 23, 42, .22); color: #172033; }
header { display: flex; justify-content: space-between; gap: 20px; align-items: start; }
header p { margin: 0 0 4px; color: #0891b2; font-size: 12px; font-weight: 700; letter-spacing: .08em; }
h2 { margin: 0; font-size: 22px; }
header button { border: 0; background: transparent; font-size: 26px; color: #64748b; cursor: pointer; }
.description { margin: 16px 0 20px; color: #64748b; line-height: 1.6; }
.error-message { padding: 10px 12px; border-radius: 9px; background: #fef2f2; color: #b91c1c; font-size: 13px; }
label { display: grid; gap: 7px; margin: 14px 0; color: #334155; font-size: 13px; font-weight: 600; }
input { min-width: 0; height: 42px; box-sizing: border-box; padding: 0 12px; border: 1px solid #cbd5e1; border-radius: 9px; font: inherit; }
.captcha-row { display: grid; grid-template-columns: 1fr 112px; gap: 10px; }
.captcha-row img { width: 112px; height: 42px; object-fit: contain; border-radius: 9px; background: #f8fafc; cursor: pointer; }
.primary, .secondary { width: 100%; height: 42px; margin-top: 8px; border-radius: 9px; font-weight: 700; cursor: pointer; }
.primary { border: 0; background: #0891b2; color: white; }
.secondary { border: 1px solid #cbd5e1; background: white; color: #475569; }
button:disabled { opacity: .55; cursor: not-allowed; }
</style>

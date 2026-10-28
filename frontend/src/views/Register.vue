<template>
  <AuthLayout class="register-page" title="注册账号" :target-name="targetName" fixed-mobile>
    <div v-if="errorMessage" class="auth-notice" role="alert">{{ errorMessage }}</div>
    <el-form ref="formRef" :model="form" :rules="rules" @submit.prevent="handleRegister">
      <el-form-item prop="email">
        <label class="auth-label" for="register-email">邮箱</label>
        <el-input id="register-email" v-model="form.email" type="email" autocomplete="username" placeholder="请输入邮箱" :disabled="busy" />
      </el-form-item>
      <el-form-item prop="password">
        <label class="auth-label" for="register-password">密码</label>
        <el-input id="register-password" v-model="form.password" :type="showPassword ? 'text' : 'password'"
          autocomplete="new-password" placeholder="至少10位，含英文和数字" :disabled="busy">
          <template #suffix>
            <button type="button" class="auth-icon-button" :disabled="busy" :aria-label="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword">
              <EyeOff v-if="showPassword" /><Eye v-else />
            </button>
          </template>
        </el-input>
      </el-form-item>
      <el-form-item prop="confirmPassword">
        <label class="auth-label" for="register-confirm-password">确认密码</label>
        <el-input id="register-confirm-password" v-model="form.confirmPassword" :type="showConfirmPassword ? 'text' : 'password'"
          autocomplete="new-password" placeholder="请再次输入密码" :disabled="busy">
          <template #suffix>
            <button type="button" class="auth-icon-button" :disabled="busy" :aria-label="showConfirmPassword ? '隐藏确认密码' : '显示确认密码'" @click="showConfirmPassword = !showConfirmPassword">
              <EyeOff v-if="showConfirmPassword" /><Eye v-else />
            </button>
          </template>
        </el-input>
      </el-form-item>
      <el-form-item prop="code">
        <label class="auth-label" for="register-code">邮箱验证码</label>
        <el-input id="register-code" v-model="code" maxlength="6" inputmode="numeric" autocomplete="one-time-code" placeholder="6 位验证码" :disabled="busy">
          <template #suffix>
            <button type="button" class="auth-code-button" :disabled="busy || countdown > 0" @click="openCodeModal">
              {{ countdown > 0 ? `${countdown}秒后重发` : '获取验证码' }}
            </button>
          </template>
        </el-input>
      </el-form-item>
      <button type="submit" class="auth-primary" :disabled="busy">
        <LoaderCircle v-if="busy" class="w-4 h-4 animate-spin" />{{ busy ? '注册中…' : '注册' }}
      </button>
    </el-form>
    <template #footer>
      <p class="auth-switch-link">已有账号？<router-link :to="loginRoute">立即登录</router-link></p>
      <AuthProviders action="注册" :disabled="busy" @select="oauthRegister" />
    </template>
  </AuthLayout>
  <SendCodeModal :visible="codeModalVisible" :email="form.email" purpose="REGISTER" @close="codeModalVisible = false" @success="acceptCode" />
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Eye, EyeOff, LoaderCircle } from 'lucide-vue-next'
import { authAPI, setAccessToken } from '../api/auth'
import AuthLayout from '../components/AuthLayout.vue'
import AuthProviders from '../components/AuthProviders.vue'
import SendCodeModal from '../components/SendCodeModal.vue'
import { getLoginRedirect, loadTargetClientName } from '../utils/oauthTarget'
import { normalizeAuthEmail, useAuthSubmission, useEmailChallenge, validAuthEmail } from '../utils/authState'
import { passwordPolicyError } from '../utils/passwordPolicy'

const route = useRoute()
const redirectUrl = getLoginRedirect(route)
const targetName = ref('账号中心')
const loginRoute = { path: '/login', query: redirectUrl === '/profile' ? {} : { redirect: redirectUrl } }
const errorMessage = ref('')
const showPassword = ref(false)
const showConfirmPassword = ref(false)
const formRef = ref(null)
const form = reactive({ email: '', password: '', confirmPassword: '' })
const { busy, run } = useAuthSubmission()
const { code, challengeId, valid: challengeValid, countdown, visible: codeModalVisible, accept: acceptCode } = useEmailChallenge(() => form.email)
const rules = computed(() => ({
  email: [{ validator: (_rule, value, callback) => callback(validAuthEmail(value) ? undefined : new Error('请输入正确的邮箱地址')), trigger: 'blur' }],
  password: [{ validator: (_rule, value, callback) => callback(passwordPolicyError(value) ? new Error(passwordPolicyError(value)) : undefined), trigger: 'blur' }],
  confirmPassword: [{ validator: (_rule, value, callback) => callback(value && value === form.password ? undefined : new Error('两次输入的密码不一致')), trigger: 'blur' }],
  code: [{ validator: (_rule, _value, callback) => callback(/^\d{6}$/.test(code.value) ? undefined : new Error('请输入六位验证码')), trigger: 'blur' }]
}))
const openCodeModal = () => {
  if (busy.value || countdown.value > 0) return
  if (!validAuthEmail(form.email)) { errorMessage.value = '请输入正确的邮箱地址'; return }
  errorMessage.value = ''
  codeModalVisible.value = true
}
const handleRegister = () => run(async isCurrent => {
  errorMessage.value = ''
  try { await formRef.value.validate() } catch { return }
  if (!isCurrent()) return
  if (!challengeValid.value) { errorMessage.value = '请先获取有效的邮箱验证码'; return }
  try {
    const response = await authAPI.register({ email: normalizeAuthEmail(form.email), password: form.password, challenge_id: challengeId.value, code: code.value, remember_me: true })
    if (!isCurrent()) return
    const data = response.data || response
    setAccessToken(data.access_token)
    window.location.assign(redirectUrl)
    return true
  } catch (error) {
    if (isCurrent()) errorMessage.value = error.message || '注册失败'
  }
})
const oauthRegister = provider => {
  if (busy.value) return
  busy.value = true
  window.location.assign(`/api/auth/third/${provider}?redirect=${encodeURIComponent(redirectUrl)}&remember_me=true`)
}
watch(() => form.email, () => { errorMessage.value = '' })
onMounted(async () => { targetName.value = await loadTargetClientName(redirectUrl) || '账号中心' })
</script>

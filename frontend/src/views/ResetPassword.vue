<template>
  <AuthLayout title="找回密码" :target-name="targetName">
    <p class="reset-description">通过邮箱验证码确认身份后，设置新的登录密码。</p>
    <div v-if="errorMessage" class="auth-notice" role="alert">{{ errorMessage }}</div>
    <div v-if="successMessage" class="auth-notice auth-notice--success" role="status">{{ successMessage }}</div>
    <el-form ref="formRef" :model="form" :rules="rules" @submit.prevent="handleReset">
      <el-form-item prop="email">
        <label class="auth-label" for="reset-email">邮箱</label>
        <el-input id="reset-email" v-model="form.email" type="email" autocomplete="username" placeholder="请输入邮箱" :disabled="busy" />
      </el-form-item>
      <el-form-item prop="code">
        <label class="auth-label" for="reset-code">邮箱验证码</label>
        <el-input id="reset-code" v-model="code" maxlength="6" inputmode="numeric" autocomplete="one-time-code" placeholder="6 位验证码" :disabled="busy">
          <template #suffix>
            <button type="button" class="auth-code-button" :disabled="busy || countdown > 0" @click="openCodeModal">
              {{ countdown > 0 ? `${countdown}秒后重发` : '获取验证码' }}
            </button>
          </template>
        </el-input>
      </el-form-item>
      <el-form-item prop="password">
        <label class="auth-label" for="reset-password">新密码</label>
        <el-input id="reset-password" v-model="form.password" :type="showPassword ? 'text' : 'password'"
          autocomplete="new-password" placeholder="至少10位，含英文和数字" :disabled="busy">
          <template #suffix>
            <button type="button" class="auth-icon-button" :disabled="busy" :aria-label="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword">
              <EyeOff v-if="showPassword" /><Eye v-else />
            </button>
          </template>
        </el-input>
      </el-form-item>
      <el-form-item prop="confirmPassword">
        <label class="auth-label" for="reset-confirm-password">确认新密码</label>
        <el-input id="reset-confirm-password" v-model="form.confirmPassword" :type="showConfirmPassword ? 'text' : 'password'"
          autocomplete="new-password" placeholder="请再次输入新密码" :disabled="busy">
          <template #suffix>
            <button type="button" class="auth-icon-button" :disabled="busy" :aria-label="showConfirmPassword ? '隐藏确认密码' : '显示确认密码'" @click="showConfirmPassword = !showConfirmPassword">
              <EyeOff v-if="showConfirmPassword" /><Eye v-else />
            </button>
          </template>
        </el-input>
      </el-form-item>
      <button type="submit" class="auth-primary" :disabled="busy">
        <LoaderCircle v-if="busy && !successMessage" class="w-4 h-4 animate-spin" />
        {{ successMessage ? '已重置' : busy ? '提交中…' : '重置密码' }}
      </button>
    </el-form>
    <template #footer>
      <p class="auth-switch-link">想起密码了？<router-link :to="loginRoute">返回登录</router-link></p>
    </template>
  </AuthLayout>
  <SendCodeModal :visible="codeModalVisible" :email="form.email" purpose="PASSWORD_RESET" @close="codeModalVisible = false" @success="acceptCode" />
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Eye, EyeOff, LoaderCircle } from 'lucide-vue-next'
import { authAPI } from '../api/auth'
import AuthLayout from '../components/AuthLayout.vue'
import SendCodeModal from '../components/SendCodeModal.vue'
import { getLoginRedirect, loadTargetClientName } from '../utils/oauthTarget'
import { normalizeAuthEmail, useAuthSubmission, useEmailChallenge, validAuthEmail } from '../utils/authState'
import { passwordPolicyError } from '../utils/passwordPolicy'

const route = useRoute()
const router = useRouter()
const redirectUrl = getLoginRedirect(route)
const targetName = ref('账号中心')
const loginRoute = { path: '/login', query: redirectUrl === '/profile' ? {} : { redirect: redirectUrl } }
const errorMessage = ref('')
const successMessage = ref('')
const showPassword = ref(false)
const showConfirmPassword = ref(false)
const formRef = ref(null)
const form = reactive({ email: '', password: '', confirmPassword: '' })
const { busy, run } = useAuthSubmission()
const { code, challengeId, valid: challengeValid, countdown, visible: codeModalVisible, accept: acceptCode } = useEmailChallenge(() => form.email)
let returnTimer
const rules = computed(() => ({
  email: [{ validator: (_rule, value, callback) => callback(validAuthEmail(value) ? undefined : new Error('请输入正确的邮箱地址')), trigger: 'blur' }],
  code: [{ validator: (_rule, _value, callback) => callback(/^\d{6}$/.test(code.value) ? undefined : new Error('请输入六位验证码')), trigger: 'blur' }],
  password: [{ validator: (_rule, value, callback) => callback(passwordPolicyError(value) ? new Error(passwordPolicyError(value)) : undefined), trigger: 'blur' }],
  confirmPassword: [{ validator: (_rule, value, callback) => callback(value && value === form.password ? undefined : new Error('两次输入的密码不一致')), trigger: 'blur' }]
}))
const openCodeModal = () => {
  if (busy.value || countdown.value > 0) return
  if (!validAuthEmail(form.email)) { errorMessage.value = '请输入正确的邮箱地址'; return }
  errorMessage.value = ''
  codeModalVisible.value = true
}
const handleReset = () => run(async isCurrent => {
  errorMessage.value = ''
  try { await formRef.value.validate() } catch { return }
  if (!isCurrent()) return
  if (!challengeValid.value) { errorMessage.value = '请先获取有效的邮箱验证码'; return }
  try {
    await authAPI.resetPassword({ email: normalizeAuthEmail(form.email), challenge_id: challengeId.value, code: code.value, password: form.password })
    if (!isCurrent()) return
    successMessage.value = '密码已重置，正在返回登录页…'
    returnTimer = setTimeout(() => { void router.replace(loginRoute) }, 1000)
    return true
  } catch (error) {
    if (isCurrent()) errorMessage.value = error.message || '重置密码失败'
  }
})
watch(() => form.email, () => { errorMessage.value = '' })
onMounted(async () => { targetName.value = await loadTargetClientName(redirectUrl) || '账号中心' })
onUnmounted(() => { clearTimeout(returnTimer) })
</script>

<style scoped>
.reset-description { margin: -12px 0 24px; color: #8a8a8a; font-size: 13px; line-height: 1.6; }
@media (max-height: 700px) { .reset-description { margin: -4px 0 20px; } }
</style>

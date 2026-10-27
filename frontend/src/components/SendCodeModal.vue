<template>
  <el-dialog :model-value="visible" title="获取验证码" width="400px" class="auth-theme auth-code-dialog" append-to-body
    :close-on-click-modal="false" @update:model-value="value => { if (!value) close() }">
    <div v-if="error" class="auth-notice" role="alert">{{ error }}</div>
    <label class="auth-label" for="send-code-captcha">图形验证码</label>
    <div class="auth-captcha-row">
      <el-input id="send-code-captcha" v-model="captcha" maxlength="4" inputmode="numeric" autocomplete="off"
        placeholder="四位数字" :disabled="loading || fetching" @keyup.enter="submit" />
      <button type="button" class="auth-captcha-refresh" :disabled="loading || fetching" aria-label="刷新图形验证码" @click="load">
        <img v-if="image" :src="image" alt="图形验证码" /><span v-else>{{ fetching ? '加载中…' : '点击重试' }}</span>
      </button>
    </div>
    <p class="text-xs text-neutral-400 mt-2">点击图片刷新</p>
    <div class="auth-dialog-actions">
      <button type="button" class="auth-secondary" @click="close">取消</button>
      <button type="button" class="auth-primary" :disabled="loading || fetching" @click="submit">{{ loading ? '发送中…' : '发送' }}</button>
    </div>
  </el-dialog>
</template>

<script setup>
import { authAPI } from '../api/auth'
import { useCodeDelivery } from '../utils/authState'
import '../auth.css'

const props = defineProps({
  visible: { type: Boolean, default: false },
  email: { type: String, default: '' },
  purpose: { type: String, default: 'LOGIN' }
})
const emit = defineEmits(['close', 'success'])
const { image, captcha, error, loading, fetching, load, close, submit } = useCodeDelivery({
  context: () => props,
  api: authAPI,
  onSuccess: data => emit('success', data),
  onClose: () => emit('close')
})
</script>

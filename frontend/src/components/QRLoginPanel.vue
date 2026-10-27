<template>
  <section class="qr-login-panel" aria-label="扫码登录">
    <h2>扫码登录</h2>
    <div class="qr-image">
      <img v-if="state.image" :src="state.image" alt="扫码登录二维码" width="176" height="176" />
      <span v-else-if="state.loading">正在加载…</span>
      <button v-else type="button" class="qr-retry" @click="start">
        {{ state.expired ? '二维码已过期' : '二维码暂不可用' }}<br /><span>点击{{ state.expired ? '刷新' : '重试' }}</span>
      </button>
    </div>
    <p role="status">{{ state.message }}</p>
    <p class="qr-countdown">{{ state.seconds > 0 ? `有效期 ${Math.floor(state.seconds / 60)}:${String(state.seconds % 60).padStart(2, '0')}` : '\u00a0' }}</p>
  </section>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import QRCode from 'qrcode'
import { authAPI } from '../api/auth'
import { createQRLoginController } from '../utils/qrLogin'

const props = defineProps({ redirect: { type: String, required: true } })
const emit = defineEmits(['complete'])
const state = ref({ image: '', message: '', seconds: 0, loading: true, expired: false })
const controller = createQRLoginController({
  api: authAPI,
  render: payload => QRCode.toDataURL(payload, { width: 352, margin: 4, errorCorrectionLevel: 'M' }),
  onChange: next => { state.value = next },
  onComplete: response => emit('complete', response)
})
const start = () => controller.start(props.redirect)
const stop = () => controller.stop()
onMounted(() => { window.addEventListener('pagehide', stop); void start() })
onUnmounted(() => { window.removeEventListener('pagehide', stop); stop() })
</script>

<style scoped>
.qr-login-panel { min-height: var(--login-method-height); text-align: center; display: flex; align-items: center; flex-direction: column; }
h2 { height: 45px; align-self: stretch; text-align: left; font-size: 15px; font-weight: 600; }
.qr-image { width: 176px; height: 176px; background: #fafafa; display: flex; align-items: center; justify-content: center; border-radius: 6px; color: #888; font-size: 13px; }
.qr-retry { width: 100%; height: 100%; line-height: 2; }
.qr-retry span { text-decoration: underline; text-underline-offset: 3px; color: #171717; }
p { margin-top: 10px; font-size: 12px; color: #737373; line-height: 18px; }
.qr-countdown { margin-top: 2px; color: #a3a3a3; }
</style>

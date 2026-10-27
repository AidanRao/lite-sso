<template>
  <section class="qr-login-panel" aria-label="扫码登录">
    <div class="qr-image">
      <img v-if="state.image" :src="state.image" alt="扫码登录二维码" width="240" height="240" />
      <span v-else-if="state.loading">正在加载…</span>
      <button v-else type="button" class="qr-retry" @click="start">
        {{ state.expired ? '二维码已过期' : '二维码暂不可用' }}<br /><span>点击{{ state.expired ? '刷新' : '重试' }}</span>
      </button>
    </div>
    <p role="status">
      <template v-if="state.message === '请使用 BUAA ClassHopper 扫码'">
        请使用 <a class="qr-help-link" href="https://aidanrao.top/projects?projectId=3" target="_blank" rel="noopener noreferrer">BUAA ClassHopper</a> 扫码
      </template>
      <template v-else>{{ state.message }}</template>
    </p>
    <p class="qr-countdown">{{ state.seconds > 0 ? `有效期 ${Math.floor(state.seconds / 60)}:${String(state.seconds % 60).padStart(2, '0')}` : '\u00a0' }}</p>
  </section>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import QRCode from 'qrcode'
import { authAPI } from '../api/auth'
import { createQRLoginController } from '../utils/qrLogin'

const props = defineProps({ redirect: { type: String, required: true } })
const emit = defineEmits(['complete', 'busy'])
const state = ref({ image: '', message: '', seconds: 0, loading: true, expired: false })
const controller = createQRLoginController({
  api: authAPI,
  render: payload => QRCode.toDataURL(payload, { width: 480, margin: 4, errorCorrectionLevel: 'M' }),
  onChange: next => { state.value = next; emit('busy', next.loading && Boolean(next.image)) },
  onComplete: response => emit('complete', response)
})
const start = () => controller.start(props.redirect)
const stop = () => controller.stop()
onMounted(() => { window.addEventListener('pagehide', stop); void start() })
onUnmounted(() => { window.removeEventListener('pagehide', stop); stop() })
</script>

<style scoped>
.qr-login-panel { min-height: calc(var(--login-method-height) + var(--qr-registration-space)); text-align: center; display: flex; align-items: center; justify-content: center; flex-direction: column; }
.qr-image { width: 240px; height: 240px; background: #fafafa; display: flex; align-items: center; justify-content: center; border-radius: 6px; color: #888; font-size: 13px; }
.qr-retry { width: 100%; height: 100%; line-height: 2; }
.qr-retry span { text-decoration: underline; text-underline-offset: 3px; color: #171717; }
p { margin-top: 10px; font-size: 12px; color: #737373; line-height: 18px; }
.qr-help-link { text-decoration: underline; text-underline-offset: 3px; }
.qr-countdown { margin-top: 2px; color: #a3a3a3; }
</style>

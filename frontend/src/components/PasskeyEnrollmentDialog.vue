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
        <PasskeyEnrollmentForm @registered="handleRegistered" @submitting="submitting = $event" />
      </section>
    </div>
  </Teleport>
</template>

<script setup>
import { ref } from 'vue'
import PasskeyEnrollmentForm from './PasskeyEnrollmentForm.vue'

defineProps({ visible: { type: Boolean, default: false } })
const emit = defineEmits(['close', 'registered'])
const submitting = ref(false)

const close = () => {
  if (!submitting.value) emit('close')
}

const handleRegistered = () => {
  emit('registered')
  emit('close')
}
</script>

<style scoped>
.passkey-mask { position: fixed; inset: 0; z-index: 80; display: grid; place-items: center; padding: 20px; background: rgba(15, 23, 42, .55); backdrop-filter: blur(4px); }
.passkey-dialog { width: min(440px, 100%); box-sizing: border-box; padding: 26px; border-radius: 18px; background: white; box-shadow: 0 28px 70px rgba(15, 23, 42, .22); color: #1f2328; }
header { display: flex; align-items: start; justify-content: space-between; gap: 20px; }
header p { margin: 0 0 4px; color: #0969da; font-size: 12px; font-weight: 700; letter-spacing: .08em; }
h2 { margin: 0; font-size: 22px; }
header button { border: 0; background: transparent; color: #57606a; font-size: 26px; cursor: pointer; }
header button:disabled { cursor: not-allowed; opacity: .55; }
</style>

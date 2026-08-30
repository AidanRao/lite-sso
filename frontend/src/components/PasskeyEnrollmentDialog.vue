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
.passkey-mask { position: fixed; inset: 0; z-index: 80; display: grid; place-items: center; padding: 20px; background: var(--profile-overlay); backdrop-filter: blur(4px); }
.passkey-dialog { width: min(440px, 100%); box-sizing: border-box; padding: 26px; border: 1px solid var(--profile-border); border-radius: 18px; background: var(--profile-surface-subtle); box-shadow: var(--profile-shadow); color: var(--profile-text); }
header { display: flex; align-items: start; justify-content: space-between; gap: 20px; }
header p { margin: 0 0 4px; color: var(--profile-accent); font-size: 12px; font-weight: 700; letter-spacing: .08em; }
h2 { margin: 0; font-size: 22px; }
header button { border: 0; background: transparent; color: var(--profile-text-muted); font-size: 26px; cursor: pointer; }
header button:disabled { cursor: not-allowed; opacity: .55; }
</style>

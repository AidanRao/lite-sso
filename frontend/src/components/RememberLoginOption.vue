<template>
  <label class="remember-login-option">
    <input type="checkbox" :checked="modelValue" :disabled="disabled" @change="$emit('update:modelValue', $event.target.checked)" />
    <span>记住登录状态</span>
  </label>
</template>

<script setup>
defineProps({ modelValue: { type: Boolean, required: true }, disabled: { type: Boolean, default: false } })
defineEmits(['update:modelValue'])
</script>

<style scoped>
.remember-login-option { display: inline-flex; align-items: center; gap: 7px; color: #525252; font-size: 13px; cursor: pointer; }
input { width: 15px; height: 15px; flex-shrink: 0; accent-color: #111; cursor: inherit; }
input:focus-visible { outline: 2px solid #737373; outline-offset: 3px; }
.remember-login-option:has(input:disabled) { cursor: not-allowed; opacity: .6; }
</style>

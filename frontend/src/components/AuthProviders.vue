<template>
  <div class="auth-providers-divider">其他{{ action }}方式</div>
  <div class="auth-providers">
    <button v-for="provider in providers" :key="provider.id" type="button" class="auth-provider"
      :disabled="disabled" :aria-label="`${provider.name}${action}`" @click="$emit('select', provider.id)">
      <span class="auth-provider-icon"><ThirdPartyProviderIcon :provider="provider.id" :size="24" /></span>
      <span>{{ provider.name }}</span>
    </button>
  </div>
</template>

<script setup>
import ThirdPartyProviderIcon from './ThirdPartyProviderIcon.vue'

defineProps({ action: { type: String, default: '登录' }, disabled: { type: Boolean, default: false } })
defineEmits(['select'])
const providers = [
  { id: 'github', name: 'GitHub' },
  { id: 'feishu', name: '飞书' },
  { id: 'qq', name: 'QQ' },
  { id: 'google', name: 'Google' }
]
</script>

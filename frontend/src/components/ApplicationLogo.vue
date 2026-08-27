<template>
  <span class="application-logo" :class="sizeClass" :aria-label="`${label || '应用'} Logo`">
    <img v-if="src" :src="src" :alt="`${label || '应用'} Logo`" />
    <span v-else>{{ initial }}</span>
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: {
    type: String,
    default: ''
  },
  src: {
    type: String,
    default: ''
  },
  size: {
    type: String,
    default: 'medium'
  }
})

const initial = computed(() => (props.label || '?').trim().slice(0, 1).toUpperCase() || '?')
const sizeClass = computed(() => `application-logo--${props.size}`)
</script>

<style scoped>
.application-logo {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid #dbe4ee;
  border-radius: 8px;
  background: #f0fdfa;
  color: #0f766e;
  font-weight: 800;
  line-height: 1;
}

.application-logo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.application-logo--small {
  width: 28px;
  height: 28px;
  font-size: 12px;
}

.application-logo--medium {
  width: 36px;
  height: 36px;
  font-size: 15px;
}

.application-logo--large {
  width: 44px;
  height: 44px;
  font-size: 18px;
}
</style>

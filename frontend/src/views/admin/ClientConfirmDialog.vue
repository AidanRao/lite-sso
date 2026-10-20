<template>
  <dialog ref="dialog" class="client-confirm-dialog" :aria-label="title" @cancel.prevent="!busy && emit('close')">
    <header><h2>{{ title }}</h2></header>
    <div class="confirm-copy"><slot /></div>
    <p v-if="error" class="admin-error" role="alert">{{ error }}</p>
    <footer>
      <button ref="cancelButton" class="admin-button" type="button" :disabled="busy" @click="emit('close')">取消</button>
      <button class="admin-button danger-text" type="button" :disabled="busy" @click="emit('confirm')">{{ busy ? '正在处理…' : confirmText }}</button>
    </footer>
  </dialog>
</template>

<script setup>
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
const props = defineProps({ open: Boolean, busy: Boolean, title: String, confirmText: String, error: String })
const emit = defineEmits(['close', 'confirm'])
const dialog = ref(null)
const cancelButton = ref(null)
watch(() => props.open, async open => {
  await nextTick()
  if (open) { dialog.value?.showModal(); cancelButton.value?.focus() }
  else dialog.value?.close()
}, { immediate: true })
onBeforeUnmount(() => dialog.value?.close())
</script>

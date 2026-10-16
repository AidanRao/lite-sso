<template>
  <button
    ref="trigger"
    class="account-menu-trigger"
    type="button"
    :aria-label="`${providerName} 更多操作`"
    aria-haspopup="menu"
    :aria-expanded="open"
    :aria-controls="open ? menuID : undefined"
    :disabled="busy"
    @click="open ? close() : show()"
    @keydown.down.up.prevent="show()"
    @keydown.esc.stop.prevent="close()"
  >
    <Ellipsis :size="20" aria-hidden="true" />
  </button>
  <Teleport to="body">
    <div
      v-if="open"
      :id="menuID"
      ref="panel"
      class="account-menu"
      :style="position"
      role="menu"
      :aria-label="`${providerName} 更多操作`"
      tabindex="-1"
      @keydown="onKeydown"
    >
      <button
        class="account-menu-danger"
        type="button"
        role="menuitem"
        tabindex="-1"
        :disabled="disabled"
        :title="disabled ? '至少需要保留一种可用登录方式' : undefined"
        @click="unbind"
      >
        <Unlink :size="16" aria-hidden="true" />
        解绑
      </button>
    </div>
  </Teleport>
</template>

<script setup>
import { nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'
import { Ellipsis, Unlink } from 'lucide-vue-next'

const props = defineProps({
  providerName: { type: String, required: true },
  disabled: { type: Boolean, default: false },
  busy: { type: Boolean, default: false }
})
const emit = defineEmits(['unbind'])
const trigger = ref(null)
const panel = ref(null)
const open = ref(false)
const position = ref({ visibility: 'hidden' })
const menuID = `third-party-account-menu-${useId()}`

const updatePosition = () => {
  if (!open.value || !trigger.value || !panel.value) return
  const anchor = trigger.value.getBoundingClientRect()
  const viewportWidth = document.documentElement.clientWidth
  const viewportHeight = window.innerHeight
  if (anchor.bottom < 0 || anchor.top > viewportHeight) { close(false); return }
  const width = Math.min(180, viewportWidth - 24)
  const height = panel.value.offsetHeight
  const top = anchor.bottom + height + 6 > viewportHeight - 12 ? anchor.top - height - 6 : anchor.bottom + 6
  position.value = {
    width: `${width}px`,
    left: `${Math.max(12, Math.min(anchor.right - width, viewportWidth - width - 12))}px`,
    top: `${Math.max(12, top)}px`
  }
}
const outside = event => {
  if (!trigger.value?.contains(event.target) && !panel.value?.contains(event.target)) close(false)
}
const removeListeners = () => {
  document.removeEventListener('pointerdown', outside, true)
  document.removeEventListener('focusin', outside)
  document.removeEventListener('scroll', updatePosition, true)
  window.removeEventListener('resize', updatePosition)
}
const close = (restoreFocus = true) => {
  if (!open.value) return
  open.value = false
  removeListeners()
  if (restoreFocus) trigger.value?.focus()
}
const focusAction = () => {
  const target = panel.value?.querySelector('button:not(:disabled)') || panel.value
  target?.focus()
}
const show = async () => {
  if (props.busy || open.value) return
  position.value = { visibility: 'hidden' }
  open.value = true
  await nextTick()
  if (!open.value || !panel.value) return
  updatePosition()
  await nextTick()
  if (!open.value || !panel.value) return
  focusAction()
  document.addEventListener('pointerdown', outside, true)
  document.addEventListener('focusin', outside)
  document.addEventListener('scroll', updatePosition, true)
  window.addEventListener('resize', updatePosition)
}
const onKeydown = event => {
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    close()
  } else if (event.key === 'Tab') {
    close()
  } else if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
    event.preventDefault()
    focusAction()
  }
}
const unbind = () => {
  if (props.disabled || props.busy) return
  close()
  emit('unbind')
}

watch(() => props.busy, busy => { if (busy) close(false) })
onBeforeUnmount(removeListeners)
</script>

<style scoped>
.account-menu-trigger { display: inline-flex; width: 34px; height: 34px; align-items: center; justify-content: center; border: 0; border-radius: 6px; background: transparent; color: var(--profile-text-muted); cursor: pointer; }
.account-menu-trigger:hover:not(:disabled), .account-menu-trigger[aria-expanded=true] { background: var(--profile-surface-hover); }
.account-menu-trigger:disabled { cursor: wait; opacity: .55; }
.account-menu { position: fixed; z-index: 3000; box-sizing: border-box; padding: 6px; border: 1px solid var(--profile-border); border-radius: 6px; background: var(--profile-surface); color: var(--profile-text-strong); box-shadow: var(--profile-shadow-soft); }
.account-menu-danger { display: flex; width: 100%; min-height: 34px; align-items: center; gap: 8px; padding: 7px 10px; border: 0; border-radius: 4px; background: transparent; color: var(--profile-danger); font: inherit; font-size: 13px; font-weight: 600; cursor: pointer; }
.account-menu-danger:hover:not(:disabled) { background: var(--profile-danger-soft); }
.account-menu-danger:disabled { color: var(--profile-text-faint); cursor: not-allowed; }
.account-menu-trigger:focus-visible, .account-menu-danger:focus-visible, .account-menu:focus-visible { outline: 2px solid var(--profile-accent); outline-offset: 2px; }
</style>

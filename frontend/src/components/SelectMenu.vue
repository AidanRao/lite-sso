<template>
  <span ref="root" class="select-menu" v-bind="$attrs">
    <button
      :id="id"
      ref="trigger"
      class="select-trigger"
      type="button"
      :disabled="disabled"
      :aria-label="`${label}：${displayLabel}`"
      aria-haspopup="listbox"
      :aria-expanded="open"
      :aria-controls="open ? menuID : undefined"
      @click="toggle"
      @keydown.down.prevent="show()"
      @keydown.up.prevent="show(true)"
      @keydown.esc.stop.prevent="close()"
    >
      <span class="trigger-label">{{ displayLabel }}</span>
      <ChevronDown :size="13" aria-hidden="true" />
    </button>
  </span>
  <Teleport to="body">
    <div v-if="open" ref="panel" class="select-popover" :style="position" @keydown="onKeydown">
      <header v-if="title || needsConfirmation" class="select-header">
        <strong>{{ title || label }}</strong>
        <button class="close-button" type="button" aria-label="关闭选择器" @click="close()"><X :size="16" aria-hidden="true" /></button>
      </header>
      <div :id="menuID" class="select-options" role="listbox" :aria-label="title || label" :aria-multiselectable="multiple || undefined">
        <button
          v-for="(option, index) in options"
          :key="option.value"
          class="select-option"
          type="button"
          role="option"
          tabindex="-1"
          :aria-selected="isSelected(option.value)"
          :disabled="option.disabled"
          @click="choose(option.value)"
          @focus="activeIndex = index"
        >
          <span class="selection-mark" :class="{ checkbox: multiple, checked: isSelected(option.value) }" aria-hidden="true"><Check v-if="isSelected(option.value)" :size="15" /></span>
          <span class="option-label">{{ option.label }}</span>
        </button>
        <p v-if="!options.length" class="empty-options">暂无可选项</p>
      </div>
      <footer v-if="needsConfirmation" class="select-footer">
        <button class="action-button" type="button" @click="close()">{{ cancelText }}</button>
        <button class="action-button confirm-button" type="button" @click="commit()">{{ confirmText }}</button>
      </footer>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'
import { Check, ChevronDown, X } from 'lucide-vue-next'

defineOptions({ inheritAttrs: false })
const props = defineProps({
  modelValue: { type: [String, Number, Array], default: '' },
  options: { type: Array, default: () => [] },
  id: { type: String, default: undefined },
  label: { type: String, default: '选择选项' },
  placeholder: { type: String, default: '请选择' },
  title: { type: String, default: '' },
  multiple: { type: Boolean, default: false },
  confirm: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
  confirmText: { type: String, default: '确定' },
  cancelText: { type: String, default: '取消' }
})
const emit = defineEmits(['update:modelValue', 'change'])
const root = ref(null)
const trigger = ref(null)
const panel = ref(null)
const open = ref(false)
const draft = ref([])
const activeIndex = ref(-1)
const position = ref({ visibility: 'hidden' })
const menuID = `select-menu-${useId()}`
const needsConfirmation = computed(() => props.multiple || props.confirm)
const values = () => props.multiple ? [...(Array.isArray(props.modelValue) ? props.modelValue : [])] : [props.modelValue]
const displayLabel = computed(() => {
  const selected = props.options.filter(option => values().includes(option.value))
  if (!selected.length) return props.placeholder
  return selected.length > 1 ? `${selected[0].label} 等 ${selected.length} 项` : selected[0].label
})
const isSelected = value => draft.value.includes(value)
const focusOption = index => {
  activeIndex.value = index
  panel.value?.querySelectorAll('[role="option"]')[index]?.focus()
}
const updatePosition = () => {
  if (!open.value || !trigger.value || !panel.value) return
  const anchor = trigger.value.getBoundingClientRect()
  const viewportWidth = document.documentElement.clientWidth
  const viewportHeight = window.innerHeight
  if (anchor.bottom < 0 || anchor.top > viewportHeight || anchor.right < 0 || anchor.left > viewportWidth) { close(false); return }
  const width = Math.min(Math.max(260, anchor.width), viewportWidth - 24)
  const left = anchor.left + width > viewportWidth - 12 ? anchor.right - width : anchor.left
  const below = viewportHeight - anchor.bottom - 18
  const above = anchor.top - 18
  const upwards = below < Math.min(panel.value.scrollHeight, 240) && above > below
  const available = Math.max(80, upwards ? above : below)
  const height = Math.min(panel.value.scrollHeight, available)
  position.value = {
    width: `${width}px`,
    left: `${Math.max(12, Math.min(left, viewportWidth - width - 12))}px`,
    top: `${upwards ? Math.max(12, anchor.top - height - 6) : anchor.bottom + 6}px`,
    maxHeight: `${available}px`
  }
}
const close = (restoreFocus = true) => {
  if (!open.value) return
  open.value = false
  removeListeners()
  if (restoreFocus) trigger.value?.focus()
}
const outside = event => {
  if (!root.value?.contains(event.target) && !panel.value?.contains(event.target)) close(false)
}
const onScroll = event => { if (!panel.value?.contains(event.target)) updatePosition() }
const removeListeners = () => {
  document.removeEventListener('pointerdown', outside, true)
  document.removeEventListener('focusin', outside)
  document.removeEventListener('scroll', onScroll, true)
  window.removeEventListener('resize', updatePosition)
}
const show = async (fromEnd = false) => {
  if (props.disabled || open.value) return
  draft.value = values()
  position.value = { visibility: 'hidden' }
  open.value = true
  await nextTick()
  if (!open.value || !panel.value) return
  updatePosition()
  await nextTick()
  if (!open.value || !panel.value) return
  updatePosition()
  const enabled = props.options.map((option, index) => option.disabled ? -1 : index).filter(index => index >= 0)
  const selected = enabled.find(index => isSelected(props.options[index].value))
  focusOption(fromEnd ? enabled.at(-1) : (selected ?? enabled[0]))
  document.addEventListener('pointerdown', outside, true)
  document.addEventListener('focusin', outside)
  document.addEventListener('scroll', onScroll, true)
  window.addEventListener('resize', updatePosition)
}
const toggle = () => open.value ? close() : show()
const commit = () => {
  const value = props.multiple ? [...draft.value] : draft.value[0]
  const previous = values()
  const changed = previous.length !== draft.value.length || previous.some(item => !draft.value.includes(item))
  close()
  if (changed) { emit('update:modelValue', value); emit('change', value) }
}
const choose = value => {
  draft.value = props.multiple
    ? isSelected(value) ? draft.value.filter(item => item !== value) : [...draft.value, value]
    : [value]
  if (!needsConfirmation.value) commit()
}
const onKeydown = event => {
  if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(); return }
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const enabled = props.options.map((option, index) => option.disabled ? -1 : index).filter(index => index >= 0)
  if (!enabled.length) return
  const current = enabled.indexOf(activeIndex.value)
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? enabled.length - 1
    : (current + (event.key === 'ArrowUp' ? -1 : 1) + enabled.length) % enabled.length
  focusOption(enabled[next])
}
watch(() => props.disabled, disabled => { if (disabled) close(false) })
onBeforeUnmount(removeListeners)
</script>

<style scoped>
.select-menu { display: inline-flex; min-width: 0; max-width: 100%; vertical-align: middle; }
.select-trigger { display: inline-flex; width: 100%; min-width: 0; min-height: 32px; align-items: center; justify-content: space-between; gap: 8px; box-sizing: border-box; padding: 5px 10px; border: 1px solid var(--profile-border-muted); border-radius: 6px; background: var(--profile-surface-subtle); color: var(--profile-text-strong); box-shadow: 0 1px 0 rgb(27 31 36 / 4%); font: inherit; font-size: 13px; line-height: 20px; text-align: left; cursor: pointer; }
.select-trigger:hover:not(:disabled), .select-trigger[aria-expanded=true] { background: var(--profile-surface-hover); }
.trigger-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.select-trigger svg { flex-shrink: 0; color: var(--profile-text-muted); }
.select-trigger:disabled { color: var(--profile-text-faint); cursor: not-allowed; opacity: .7; }
.select-popover { position: fixed; z-index: 3000; display: flex; flex-direction: column; box-sizing: border-box; overflow: hidden; border: 1px solid var(--profile-border); border-radius: 12px; background: var(--profile-surface); color: var(--profile-text-strong); box-shadow: var(--profile-shadow-soft); font-size: 14px; }
.select-options { min-height: 0; overflow-y: auto; padding: 7px; overscroll-behavior: contain; }
.select-option { display: flex; width: 100%; min-height: 32px; align-items: center; gap: 8px; padding: 6px 8px; border: 0; border-radius: 6px; background: transparent; color: inherit; font: inherit; line-height: 20px; text-align: left; cursor: pointer; }
.select-option:hover:not(:disabled), .select-option:focus-visible { background: var(--profile-surface-hover); outline: none; }
.select-option:disabled { color: var(--profile-text-faint); cursor: not-allowed; }
.option-label { min-width: 0; overflow-wrap: anywhere; }
.selection-mark { display: inline-flex; width: 16px; height: 16px; flex: 0 0 16px; align-items: center; justify-content: center; color: var(--profile-text-muted); }
.selection-mark.checkbox { box-sizing: border-box; border: 1px solid var(--profile-border-muted); border-radius: 3px; }
.selection-mark.checkbox.checked { border-color: var(--profile-accent); background: var(--profile-accent); color: #fff; }
.select-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 15px; border-bottom: 1px solid var(--profile-divider); }
.select-header strong { font-size: 14px; line-height: 20px; }
.close-button { display: inline-flex; align-items: center; justify-content: center; padding: 2px; border: 0; border-radius: 4px; background: transparent; color: var(--profile-text-muted); cursor: pointer; }
.close-button:hover { background: var(--profile-surface-hover); }
.select-footer { display: flex; justify-content: flex-end; gap: 8px; padding: 12px 15px; border-top: 1px solid var(--profile-divider); }
.action-button { min-height: 32px; padding: 5px 12px; border: 1px solid var(--profile-border-muted); border-radius: 6px; background: var(--profile-surface-subtle); color: inherit; font: inherit; font-size: 13px; font-weight: 600; cursor: pointer; }
.action-button:hover { background: var(--profile-surface-hover); }
.confirm-button { border-color: var(--profile-success-border); background: var(--profile-success-strong); color: #fff; }
.confirm-button:hover { background: var(--profile-success-hover); }
.select-trigger:focus-visible, .action-button:focus-visible, .close-button:focus-visible { outline: 2px solid var(--profile-accent); outline-offset: 2px; }
.empty-options { margin: 8px; color: var(--profile-text-muted); font-size: 13px; }
</style>

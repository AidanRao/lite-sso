<template>
  <li class="navigation-item" :class="{ 'has-children': hasChildren }">
    <RouterLink
      v-if="item.to"
      class="navigation-link"
      :class="{ active: isActive }"
      :style="indentStyle"
      :to="item.to"
      :aria-current="isActive ? 'page' : undefined"
    >
      <component :is="item.icon" v-if="item.icon" :size="17" aria-hidden="true" />
      <span>{{ item.label }}</span>
    </RouterLink>

    <button
      v-else-if="hasChildren"
      class="navigation-group-button"
      :style="indentStyle"
      type="button"
      :aria-expanded="open"
      @click="open = !open"
    >
      <span>{{ item.label }}</span>
      <ChevronDown :size="15" :class="{ rotated: !open }" aria-hidden="true" />
    </button>

    <ul v-if="hasChildren" v-show="open" class="navigation-children">
      <ProfileSidebarItem
        v-for="child in item.children"
        :key="child.to || child.label"
        :item="child"
        :depth="depth + 1"
      />
    </ul>
  </li>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ChevronDown } from 'lucide-vue-next'

defineOptions({ name: 'ProfileSidebarItem' })

const props = defineProps({
  item: {
    type: Object,
    required: true
  },
  depth: {
    type: Number,
    default: 0
  }
})

const route = useRoute()
const hasChildren = computed(() => Array.isArray(props.item.children) && props.item.children.length > 0)
const isActive = computed(() => Boolean(props.item.to) && route.path === props.item.to)
const containsActiveRoute = (item) => item.to === route.path || item.children?.some(containsActiveRoute)
const hasActiveChild = computed(() => hasChildren.value && props.item.children.some(containsActiveRoute))
const open = ref(true)
const indentStyle = computed(() => ({
  '--navigation-link-indent': `${10 + Math.max(0, props.depth - 1) * 16}px`,
  '--navigation-group-indent': `${10 + props.depth * 16}px`
}))

watch(hasActiveChild, (active) => {
  if (active) open.value = true
})
</script>

<style scoped>
.navigation-item,
.navigation-children {
  margin: 0;
  padding: 0;
  list-style: none;
}

.navigation-link,
.navigation-group-button {
  position: relative;
  display: flex;
  width: 100%;
  min-height: 34px;
  box-sizing: border-box;
  align-items: center;
  gap: 9px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--profile-text-strong);
  font: inherit;
  font-size: 14px;
  text-align: left;
}

.navigation-link {
  padding: 7px 10px 7px var(--navigation-link-indent);
  text-decoration: none;
}

.navigation-link:hover {
  background: var(--profile-surface-subtle);
}

.navigation-link.active {
  background: var(--profile-surface-hover);
  font-weight: 600;
}

.navigation-link.active::before {
  position: absolute;
  top: 5px;
  bottom: 5px;
  left: -8px;
  width: 4px;
  border-radius: 6px;
  background: var(--profile-accent);
  content: '';
}

.navigation-group-button {
  justify-content: space-between;
  margin-top: 12px;
  padding: 10px 10px 5px var(--navigation-group-indent);
  border-top: 1px solid var(--profile-divider);
  border-radius: 0;
  color: var(--profile-text-muted);
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
}

.navigation-group-button svg {
  transition: transform 0.18s ease;
}

.navigation-group-button svg.rotated {
  transform: rotate(-90deg);
}

.navigation-children {
  display: grid;
  gap: 2px;
}

@media (max-width: 760px) {
  .navigation-link.active::before {
    left: 0;
  }

  .navigation-link {
    padding-left: calc(var(--navigation-link-indent) + 4px);
  }
}
</style>

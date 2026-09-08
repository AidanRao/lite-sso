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
      <span v-if="item.beta" class="beta-badge">Beta</span>
    </RouterLink>

    <div
      v-else-if="hasChildren"
      class="navigation-group-title"
      :style="indentStyle"
    >
      <span>{{ item.label }}</span>
    </div>

    <ul v-if="hasChildren" class="navigation-children">
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
import { computed } from 'vue'
import { useRoute } from 'vue-router'

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
const indentStyle = computed(() => ({
  '--navigation-link-indent': `${10 + Math.max(0, props.depth - 1) * 16}px`,
  '--navigation-group-indent': `${10 + props.depth * 16}px`
}))
</script>

<style scoped>
.beta-badge {
  margin-left: auto;
  border: 1px solid var(--profile-border-muted);
  border-radius: 12px;
  padding: 0 7px;
  color: var(--profile-text-muted);
  font-size: 11px;
  line-height: 18px;
  font-weight: 500;
}

.navigation-item,
.navigation-children {
  margin: 0;
  padding: 0;
  list-style: none;
}

.navigation-link,
.navigation-group-title {
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

.navigation-group-title {
  margin-top: 12px;
  padding: 10px 10px 5px var(--navigation-group-indent);
  border-top: 1px solid var(--profile-divider);
  border-radius: 0;
  color: var(--profile-text-muted);
  font-size: 12px;
  font-weight: 600;
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

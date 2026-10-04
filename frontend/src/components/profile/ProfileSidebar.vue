<template>
  <aside class="settings-sidebar" aria-label="账号设置导航">
    <div class="identity">
      <div class="identity-avatar" aria-hidden="true">
        <img v-if="user?.avatar_url" :src="user.avatar_url" alt="" />
        <span v-else>{{ avatarInitial }}</span>
      </div>
      <div class="identity-copy">
        <strong>{{ displayName }}</strong>
        <span>{{ user?.email || '未设置邮箱' }}</span>
      </div>
    </div>

    <button
      ref="menuButton"
      class="menu-button"
      type="button"
      aria-controls="profile-settings-menu"
      :aria-expanded="menuOpen"
      @click="openMenu"
    >
      <PanelLeft :size="18" aria-hidden="true" />
      菜单
    </button>

    <div v-if="menuOpen" class="menu-backdrop" @click="closeMenu" />
    <div
      id="profile-settings-menu"
      ref="menuPanel"
      class="sidebar-panel"
      :class="{ 'is-open': menuOpen }"
      :role="isMobile ? 'dialog' : undefined"
      :aria-modal="isMobile && menuOpen ? 'true' : undefined"
      :aria-label="isMobile ? '设置菜单' : undefined"
      @keydown="handleMenuKeydown"
    >
      <div class="menu-header">
        <strong>设置菜单</strong>
        <button ref="closeButton" class="menu-close" type="button" aria-label="关闭菜单" @click="closeMenu">
          <X :size="20" aria-hidden="true" />
        </button>
      </div>

      <nav aria-label="账号设置页面">
        <ul class="navigation-root" @click="handleNavigationClick">
          <ProfileSidebarItem
            v-for="item in navigation"
            :key="item.to || item.label"
            :item="item"
          />
        </ul>
      </nav>

      <div class="sidebar-actions" @click="handleNavigationClick">
        <RouterLink v-if="isAdmin" class="sidebar-action" to="/admin">
          <Shield :size="16" aria-hidden="true" />
          管理后台
        </RouterLink>
        <RouterLink class="sidebar-action" to="/logout">
          <LogOut :size="16" aria-hidden="true" />
          退出登录
        </RouterLink>
      </div>
    </div>
  </aside>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { AppWindow, KeyRound, LogOut, Mail, Paintbrush, PanelLeft, RadioTower, ScrollText, Shield, UserRound, X } from 'lucide-vue-next'
import ProfileSidebarItem from './ProfileSidebarItem.vue'
import { filterFeatureNavigation } from '../../utils/features'

const props = defineProps({
  features: { type: Object, default: () => ({}) },
  user: {
    type: Object,
    default: null
  },
  isAdmin: {
    type: Boolean,
    default: false
  }
})

const navigationItems = [
  {
    label: 'Account',
    to: '/profile/account',
    icon: UserRound
  },
  {
    label: '外观',
    to: '/profile/appearance',
    icon: Paintbrush
  },
  {
    label: 'Access',
    children: [
      {
        label: 'Emails',
        to: '/profile/access/emails',
        icon: Mail
      },
      {
        label: '密码与认证',
        to: '/profile/access/authentication',
        icon: KeyRound
      },
      {
        label: 'Sessions',
        to: '/profile/access/sessions',
        icon: RadioTower
      }
    ]
  },
  {
    label: 'Integrations',
    children: [
      {
        label: '应用',
        to: '/profile/integrations/applications',
        icon: AppWindow
      }
    ]
  },
  {
    label: 'Archived',
    children: [
      { featureKey: 'profile.audit_logs', label: '操作日志', to: '/profile/archived/audit-logs', icon: ScrollText }
    ]
  }
]

const navigation = computed(() => filterFeatureNavigation(navigationItems, props.features))

const displayName = computed(() => props.user?.username || props.user?.email || 'Lite SSO 用户')
const avatarInitial = computed(() => displayName.value.slice(0, 1).toUpperCase())
const route = useRoute()
const menuOpen = ref(false)
const isMobile = ref(false)
const menuButton = ref(null)
const menuPanel = ref(null)
const closeButton = ref(null)
let mobileQuery
let previousBodyOverflow = ''

const closeMenu = (restoreFocus = true) => {
  if (!menuOpen.value) return
  menuOpen.value = false
  document.body.style.overflow = previousBodyOverflow
  if (restoreFocus && isMobile.value) nextTick(() => menuButton.value?.focus())
}

const openMenu = async () => {
  if (!isMobile.value || menuOpen.value) return
  previousBodyOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  menuOpen.value = true
  await nextTick()
  closeButton.value?.focus()
}

const handleNavigationClick = (event) => {
  if (event.target.closest('a')) closeMenu(false)
}

const handleMenuKeydown = (event) => {
  if (!isMobile.value || !menuOpen.value) return
  if (event.key === 'Escape') {
    event.preventDefault()
    closeMenu()
    return
  }
  if (event.key !== 'Tab') return
  const focusable = [...menuPanel.value.querySelectorAll('a[href], button:not([disabled])')]
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}

const handleViewportChange = () => {
  isMobile.value = mobileQuery.matches
  if (!isMobile.value) closeMenu(false)
}

watch(() => route.fullPath, () => closeMenu(false))
onMounted(() => {
  mobileQuery = window.matchMedia('(max-width: 760px)')
  handleViewportChange()
  mobileQuery.addEventListener('change', handleViewportChange)
})
onBeforeUnmount(() => {
  closeMenu(false)
  mobileQuery?.removeEventListener('change', handleViewportChange)
})
</script>

<style scoped>
.settings-sidebar {
  position: sticky;
  top: 28px;
  display: flex;
  width: 272px;
  max-height: calc(100vh - 56px);
  flex: 0 0 272px;
  flex-direction: column;
  align-self: flex-start;
}

.identity {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 12px;
  padding: 0 8px 18px;
}

.identity-avatar {
  display: flex;
  width: 48px;
  height: 48px;
  flex: 0 0 48px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid var(--profile-border-muted);
  border-radius: 50%;
  background: var(--profile-surface-subtle);
  color: var(--profile-text-muted);
  font-size: 18px;
  font-weight: 600;
}

.identity-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.identity-copy {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.identity-copy strong,
.identity-copy span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.identity-copy strong {
  color: var(--profile-text-strong);
  font-size: 16px;
}

.identity-copy span {
  color: var(--profile-text-muted);
  font-size: 13px;
}

.menu-button,
.menu-header,
.menu-backdrop {
  display: none;
}

.navigation-root {
  display: grid;
  gap: 2px;
  margin: 0;
  padding: 0 8px;
  list-style: none;
}

.sidebar-actions {
  display: grid;
  gap: 2px;
  margin: 18px 8px 0;
  padding-top: 12px;
  border-top: 1px solid var(--profile-divider);
}

.sidebar-action {
  display: flex;
  min-height: 34px;
  box-sizing: border-box;
  align-items: center;
  gap: 9px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--profile-text-strong);
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  padding: 7px 10px;
  text-decoration: none;
}

.sidebar-action:hover {
  background: var(--profile-surface-subtle);
}

@media (max-width: 760px) {
  .settings-sidebar {
    position: static;
    width: 100%;
    max-height: none;
    flex-basis: auto;
  }

  .identity {
    padding: 0 0 18px;
  }

  .menu-button {
    display: inline-flex;
    min-height: 36px;
    align-items: center;
    gap: 8px;
    align-self: flex-start;
    border: 1px solid var(--profile-border);
    border-radius: 6px;
    background: var(--profile-surface-subtle);
    color: var(--profile-text-strong);
    cursor: pointer;
    font: inherit;
    font-size: 14px;
    font-weight: 600;
    padding: 6px 12px;
  }

  .menu-button:hover,
  .menu-close:hover {
    background: var(--profile-surface-hover);
  }

  .menu-button:focus-visible,
  .menu-close:focus-visible {
    outline: 2px solid var(--profile-accent);
    outline-offset: 2px;
  }

  .menu-backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: block;
    background: var(--profile-overlay);
  }

  .sidebar-panel {
    position: fixed;
    inset: 0 auto 0 0;
    z-index: 101;
    display: none;
    width: min(360px, calc(100vw - 56px));
    box-sizing: border-box;
    overflow-y: auto;
    overscroll-behavior: contain;
    border-radius: 0 12px 12px 0;
    background: var(--profile-surface);
    box-shadow: var(--profile-shadow);
    padding: 12px 12px 24px;
  }

  .sidebar-panel.is-open {
    display: block;
  }

  .menu-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 18px;
    color: var(--profile-text-strong);
    font-size: 16px;
  }

  .menu-close {
    display: grid;
    width: 36px;
    height: 36px;
    place-items: center;
    border: 1px solid var(--profile-border);
    border-radius: 6px;
    background: var(--profile-surface-subtle);
    color: var(--profile-text-muted);
    cursor: pointer;
  }

  .navigation-root,
  .sidebar-actions {
    margin-inline: 0;
    padding-inline: 0;
  }

  .sidebar-actions {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>

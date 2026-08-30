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

    <nav>
      <ul class="navigation-root">
        <ProfileSidebarItem
          v-for="item in navigation"
          :key="item.to || item.label"
          :item="item"
        />
      </ul>
    </nav>

    <div class="sidebar-actions">
      <RouterLink v-if="isAdmin" class="sidebar-action" to="/admin">
        <Shield :size="16" aria-hidden="true" />
        管理后台
      </RouterLink>
      <button class="sidebar-action" type="button" @click="logout">
        <LogOut :size="16" aria-hidden="true" />
        退出登录
      </button>
    </div>
  </aside>
</template>

<script setup>
import { computed } from 'vue'
import { AppWindow, KeyRound, LogOut, Mail, RadioTower, Shield, UserRound } from 'lucide-vue-next'
import ProfileSidebarItem from './ProfileSidebarItem.vue'
import { submitGlobalLogout } from '../../utils/logout'

const props = defineProps({
  user: {
    type: Object,
    default: null
  },
  isAdmin: {
    type: Boolean,
    default: false
  }
})

const navigation = [
  {
    label: 'Account',
    to: '/profile/account',
    icon: UserRound
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
  }
]

const displayName = computed(() => props.user?.username || props.user?.email || 'Lite SSO 用户')
const avatarInitial = computed(() => displayName.value.slice(0, 1).toUpperCase())

const logout = () => {
  submitGlobalLogout('/login')
}
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
  border: 1px solid rgba(27, 31, 36, 0.15);
  border-radius: 50%;
  background: #f6f8fa;
  color: #57606a;
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
  color: #24292f;
  font-size: 16px;
}

.identity-copy span {
  color: #57606a;
  font-size: 13px;
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
  border-top: 1px solid #d8dee4;
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
  color: #24292f;
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  padding: 7px 10px;
  text-decoration: none;
}

.sidebar-action:hover {
  background: #f6f8fa;
}

@media (max-width: 760px) {
  .settings-sidebar {
    position: static;
    width: 100%;
    max-height: none;
    flex-basis: auto;
  }

  .identity {
    padding-inline: 0;
  }

  .navigation-root,
  .sidebar-actions {
    margin-inline: 0;
    padding-inline: 0;
  }

  .sidebar-actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>

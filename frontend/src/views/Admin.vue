<template>
  <div class="admin-page">
    <header class="admin-topbar">
      <nav class="admin-breadcrumb" aria-label="页面路径">
        <ol>
          <li><RouterLink class="admin-brand" to="/admin/users"><Shield :size="23" aria-hidden="true" /><span>系统管理</span></RouterLink></li>
          <li v-for="(item, index) in breadcrumbs" :key="item.to || 'current'">
            <RouterLink v-if="item.to" :to="item.to">{{ item.label }}</RouterLink>
            <span v-else :aria-current="index === breadcrumbs.length - 1 ? 'page' : undefined">{{ item.label }}</span>
          </li>
        </ol>
      </nav>
      <RouterLink class="back-to-profile" to="/profile"><ArrowLeft :size="15" aria-hidden="true" />返回账号设置</RouterLink>
    </header>
    <div class="admin-shell">
      <aside class="admin-sidebar">
        <span class="sidebar-caption">{{ route.meta.clientDetail ? '客户端设置' : '管理控制台' }}</span>
        <nav aria-label="管理端导航">
          <RouterLink v-for="item in navigation" :key="item.to" :to="item.to" class="admin-nav-link" :class="{ 'is-active': route.path === item.to || (!item.exact && route.path.startsWith(`${item.to}/`)) }">
            <component :is="item.icon" :size="17" aria-hidden="true" />{{ item.label }}
          </RouterLink>
        </nav>
      </aside>
      <main class="admin-content">
        <p v-if="checking" class="page-state" role="status">正在检查管理权限…</p>
        <div v-else-if="permissionError" class="admin-error" role="alert">{{ permissionError }} <button type="button" @click="checkAccess">重试</button></div>
        <RouterView v-else-if="permissions.state.isAdmin" :key="route.meta.clientDetail ? `client-${route.params.id}` : route.path" />
      </main>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { AppWindow, ArrowLeft, Settings, Shield, SlidersHorizontal, Users } from 'lucide-vue-next'
import { permissions } from '../utils/permissions'
import { useAdminAccess } from './admin/useAdminAccess'
import { CLIENT_BREADCRUMB_KEY } from './admin/clientContext'
import './admin/admin.css'

const route = useRoute()
const router = useRouter()
const clientName = ref('')
provide(CLIENT_BREADCRUMB_KEY, clientName)
watch(() => route.params.id, () => { clientName.value = '' })
const breadcrumbs = computed(() => {
  if (route.meta.clientDetail) return [
    { label: '客户端管理', to: '/admin/clients' },
    { label: clientName.value || '客户端设置' }
  ]
  if (route.name === 'AdminClientNew') return [{ label: '客户端管理', to: '/admin/clients' }, { label: '新增客户端' }]
  return [{ label: route.meta.title || '管理控制台' }]
})
const { handleAccessError } = useAdminAccess()
const managementNavigation = [
  { to: '/admin/users', label: '用户管理', icon: Users },
  { to: '/admin/clients', label: '客户端管理', icon: AppWindow },
  { to: '/admin/features', label: '功能发布', icon: SlidersHorizontal }
]
const navigation = computed(() => route.meta.clientDetail ? [
  { to: `/admin/clients/${route.params.id}`, label: 'General', icon: AppWindow, exact: true },
  { to: `/admin/clients/${route.params.id}/users`, label: 'Users', icon: Users },
  { to: `/admin/clients/${route.params.id}/advanced`, label: 'Advanced', icon: Settings }
] : managementNavigation)
const checking = ref(true)
const permissionError = ref('')
const checkAccess = async () => {
  checking.value = true
  permissionError.value = ''
  try {
    await permissions.load()
    if (!permissions.state.loaded) {
      router.replace({ path: '/login', query: { redirect: route.fullPath } })
      return
    }
    if (!permissions.state.isAdmin) {
      ElMessage.error('无管理员权限')
      router.replace('/profile')
    }
  } catch (error) {
    if (!handleAccessError(error)) permissionError.value = error.message || '管理权限加载失败'
  } finally {
    checking.value = false
  }
}
onMounted(checkAccess)
</script>

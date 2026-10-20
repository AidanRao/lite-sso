<template>
  <section aria-labelledby="clients-title">
    <header class="page-heading">
      <div>
        <h1 id="clients-title">客户端管理</h1>
      </div>
      <RouterLink class="admin-button primary" to="/admin/clients/new"><Plus :size="16" />新增客户端</RouterLink>
    </header>

    <div class="list-toolbar">
      <span>{{ clients.length }} 个客户端</span>
      <button class="admin-button" type="button" :disabled="loading" @click="loadClients"><RefreshCw :size="15" :class="{ spinning: loading }" />刷新</button>
    </div>
    <p v-if="loading" class="page-state" role="status">正在加载客户端…</p>
    <div v-else-if="loadError" class="admin-error" role="alert">{{ loadError }} <button type="button" @click="loadClients">重试</button></div>
    <div v-else-if="!clients.length" class="empty-clients">
      <AppWindow :size="30" aria-hidden="true" />
      <h2>还没有客户端</h2><p>创建客户端，配置应用信息和允许的回调地址。</p>
      <RouterLink class="admin-button" to="/admin/clients/new">新增客户端</RouterLink>
    </div>
    <ul v-else class="client-list">
      <li v-for="client in clients" :key="client.id" class="client-card">
        <div class="client-summary">
          <ApplicationLogo :label="client.name || client.client_id" :src="client.logo_url" size="small" />
          <div class="client-copy">
            <RouterLink class="client-name" :to="`/admin/clients/${client.id}`">{{ client.name || client.client_id }}</RouterLink>
            <span class="client-id mono">{{ client.client_id }}</span>
          </div>
          <span class="client-type">{{ client.client_type === 'public' ? '公共客户端 · PKCE' : '服务端客户端' }}</span>
          <RouterLink class="admin-button edit-client" :to="`/admin/clients/${client.id}`" :aria-label="`编辑 ${client.name || client.client_id}`"><Pencil :size="14" />编辑</RouterLink>
        </div>
        <dl class="client-urls">
          <div><dt>主页地址</dt><dd>{{ client.homepage_url }}</dd></div>
          <div><dt>回调地址</dt><dd><span v-for="uri in client.redirect_uris" :key="uri">{{ uri }}</span></dd></div>
        </dl>
      </li>
    </ul>
  </section>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { AppWindow, Pencil, Plus, RefreshCw } from 'lucide-vue-next'
import { adminAPI } from '../../api/auth'
import ApplicationLogo from '../../components/ApplicationLogo.vue'
import { useAdminAccess } from './useAdminAccess'

const { handleAccessError } = useAdminAccess()
const clients = ref([])
const loading = ref(true)
const loadError = ref('')

const loadClients = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const response = await adminAPI.listOAuthClients()
    clients.value = response?.data?.clients || []
  } catch (error) {
    if (!handleAccessError(error)) loadError.value = error.message || '加载客户端失败'
  } finally {
    loading.value = false
  }
}

onMounted(loadClients)
</script>

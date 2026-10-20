<template>
  <section class="client-details">
    <p v-if="loading" class="page-state" role="status">正在加载客户端…</p>
    <div v-else-if="loadError && !client" class="admin-error" role="alert">{{ loadError }} <button type="button" @click="reload().catch(() => {})">重试</button></div>
    <template v-else-if="client">
      <header v-if="route.name === 'AdminClientGeneral'" class="page-heading"><div><h1>{{ client.name }}</h1></div></header>
      <RouterView />
    </template>
  </section>
</template>

<script setup>
import { inject, onBeforeUnmount, onMounted, provide, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { adminAPI } from '../../api/auth'
import { CLIENT_BREADCRUMB_KEY, CLIENT_CONTEXT_KEY } from './clientContext'
import { useAdminAccess } from './useAdminAccess'

const route = useRoute()
const { handleAccessError } = useAdminAccess()
const client = ref(null)
const clientName = inject(CLIENT_BREADCRUMB_KEY)
watch(() => client.value?.name, name => { clientName.value = name || '' })
const loading = ref(true)
const loadError = ref('')
const id = route.params.id
let version = 0
const update = value => { client.value = { ...client.value, ...value } }
const reload = async () => {
  const current = ++version
  loading.value = !client.value
  loadError.value = ''
  try {
    const response = await adminAPI.getOAuthClient(id)
    if (current === version) client.value = response.data.client
    return true
  } catch (error) {
    if (current !== version) return false
    if (!handleAccessError(error)) loadError.value = error.message || '加载客户端失败'
    throw error
  } finally {
    if (current === version) loading.value = false
  }
}
provide(CLIENT_CONTEXT_KEY, { client, update, reload })
onMounted(() => reload().catch(() => {}))
onBeforeUnmount(() => { version++ })
</script>

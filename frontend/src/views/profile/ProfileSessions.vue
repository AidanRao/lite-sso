<template>
  <div class="settings-view">
    <section class="settings-section" aria-labelledby="sessions-title">
      <header class="section-title">
        <h2 id="sessions-title">登录设备</h2>
      </header>

      <div v-if="loading" class="section-state">
        <span class="spinner" aria-hidden="true"></span>
        正在加载登录设备…
      </div>
      <div v-else-if="errorMessage" class="section-state error-state">
        <span>{{ errorMessage }}</span>
        <button class="button" type="button" @click="loadDevices">重新加载</button>
      </div>
      <div v-else-if="deviceCards.length" class="sessions-list">
        <article v-for="device in deviceCards" :key="device.device_id" class="session-row">
          <div class="device-icon" aria-hidden="true">
            <Smartphone v-if="device.mobile" :size="21" />
            <Monitor v-else :size="21" />
          </div>
          <div class="device-content">
            <div class="device-title">
              <strong>{{ device.name }}</strong>
              <span v-if="device.current" class="current-badge">当前设备</span>
            </div>
            <span class="device-agent" :title="device.user_agent || '未知 User-Agent'">
              {{ device.user_agent || '未知 User-Agent' }}
            </span>
            <dl class="device-meta">
              <div><dt>IP</dt><dd>{{ device.ip || '-' }}</dd></div>
              <div><dt>登录方式</dt><dd>{{ authMethodLabel(device.auth_method) }}</dd></div>
              <div><dt>登录时间</dt><dd>{{ formatDate(device.created_at) }}</dd></div>
              <div><dt>最近活动</dt><dd>{{ formatDate(device.last_seen_at) }}</dd></div>
            </dl>
          </div>
          <button
            v-if="!device.current"
            class="button danger"
            type="button"
            :disabled="Boolean(revokingDevice)"
            @click="revokeDevice(device)"
          >
            {{ revokingDevice === device.device_id ? '踢出中…' : '踢出设备' }}
          </button>
        </article>
      </div>
      <div v-else class="section-state">暂无登录设备。</div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Monitor, Smartphone } from 'lucide-vue-next'
import { userAPI } from '../../api/auth'

const route = useRoute()
const router = useRouter()
const devices = ref([])
const loading = ref(true)
const errorMessage = ref('')
const revokingDevice = ref('')

const describeUserAgent = (userAgent = '') => {
  let browser = ''
  if (/Edg\//.test(userAgent)) browser = 'Edge'
  else if (/Firefox\/|FxiOS\//.test(userAgent)) browser = 'Firefox'
  else if (/Chrome\/|CriOS\//.test(userAgent)) browser = 'Chrome'
  else if (/Safari\//.test(userAgent)) browser = 'Safari'

  let system = ''
  if (/Android/.test(userAgent)) system = 'Android'
  else if (/iPhone|iPad|iPod/.test(userAgent)) system = 'iOS'
  else if (/Windows/.test(userAgent)) system = 'Windows'
  else if (/Macintosh|Mac OS X/.test(userAgent)) system = 'macOS'
  else if (/Linux/.test(userAgent)) system = 'Linux'

  return {
    name: [browser, system].filter(Boolean).join(' · ') || '未知设备',
    mobile: /Mobile|Android|iPhone|iPad|iPod/.test(userAgent)
  }
}

const deviceCards = computed(() => devices.value.map((device) => ({
  ...device,
  ...describeUserAgent(device.user_agent)
})))

const loadDevices = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    const result = await userAPI.getDevices()
    devices.value = Array.isArray(result?.data?.devices) ? result.data.devices : []
  } catch (error) {
    if (error.status === 401) {
      router.replace({ path: '/login', query: { redirect: route.fullPath } })
      return
    }
    devices.value = []
    errorMessage.value = error.message || '登录设备加载失败'
  } finally {
    loading.value = false
  }
}

const revokeDevice = async (device) => {
  if (device.current || revokingDevice.value) return
  try {
    await ElMessageBox.confirm(
      `踢出后，${device.name} 上的所有登录将立即失效，确定继续吗？`,
      '踢出登录设备',
      { confirmButtonText: '踢出设备', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }

  revokingDevice.value = device.device_id
  try {
    await userAPI.revokeDevice(device.device_id)
    devices.value = devices.value.filter((item) => item.device_id !== device.device_id)
    ElMessage.success('设备已踢出')
  } catch (error) {
    if (error.status === 401) {
      router.replace({ path: '/login', query: { redirect: route.fullPath } })
      return
    }
    if (error.status === 404) await loadDevices()
    ElMessage.error(error.message || '踢出设备失败')
  } finally {
    revokingDevice.value = ''
  }
}

const authMethodLabel = (method) => ({
  PASSWORD: '密码登录',
  EMAIL_OTP: '邮箱验证码',
  QR: '扫码登录',
  GITHUB: 'GitHub 登录',
  FEISHU: '飞书登录'
})[method] || '其他登录方式'

const formatDate = (value) => {
  if (!value) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'
  }).format(new Date(value))
}

onMounted(loadDevices)
</script>

<style scoped>
.settings-view {
  display: grid;
  gap: 22px;
}

.settings-section {
  display: grid;
  gap: 14px;
}

.section-title h2 {
  margin: 0;
  color: #24292f;
  font-size: 18px;
  font-weight: 600;
}

.current-badge {
  border: 1px solid #2da44e;
  border-radius: 999px;
  color: #1a7f37;
  font-size: 12px;
  font-weight: 600;
  padding: 2px 8px;
  white-space: nowrap;
}

.sessions-list {
  overflow: hidden;
  border: 1px solid #d0d7de;
  border-radius: 6px;
}

.session-row {
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr) auto;
  align-items: start;
  gap: 14px;
  padding: 16px;
  border-top: 1px solid #d8dee4;
}

.session-row:first-child {
  border-top: 0;
}

.device-icon {
  display: flex;
  width: 40px;
  height: 40px;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: #f6f8fa;
  color: #57606a;
}

.device-content {
  min-width: 0;
}

.device-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.device-title strong {
  font-size: 14px;
}

.device-agent {
  display: block;
  overflow: hidden;
  margin-top: 5px;
  color: #6e7781;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  margin: 12px 0 0;
}

.device-meta div {
  display: flex;
  gap: 5px;
}

.device-meta dt,
.device-meta dd {
  margin: 0;
  font-size: 12px;
}

.device-meta dt {
  color: #6e7781;
}

.device-meta dd {
  color: #24292f;
}

.section-state {
  display: flex;
  min-height: 110px;
  box-sizing: border-box;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border: 1px solid #d0d7de;
  border-radius: 6px;
  color: #57606a;
  font-size: 14px;
  padding: 16px;
}

.error-state {
  justify-content: space-between;
  color: #cf222e;
}

.spinner {
  width: 17px;
  height: 17px;
  border: 2px solid #d0d7de;
  border-top-color: #0969da;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.button {
  min-height: 32px;
  box-sizing: border-box;
  border: 1px solid rgba(27, 31, 36, 0.15);
  border-radius: 6px;
  background: #f6f8fa;
  color: #24292f;
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  padding: 5px 12px;
}

.button.danger {
  color: #cf222e;
}

.button:hover:not(:disabled) {
  background: #eaeef2;
}

.button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 620px) {
  .session-row {
    grid-template-columns: 40px minmax(0, 1fr);
  }

  .session-row > .button {
    grid-column: 2;
    justify-self: start;
  }
}
</style>

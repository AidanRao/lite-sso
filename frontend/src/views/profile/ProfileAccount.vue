<template>
  <div class="settings-view">
    <ProfileSettingsSection
      title-id="account-avatar-title"
      title="头像"
      description="支持 JPEG、PNG 或 WebP，文件大小不超过 2MB。"
    >
      <div class="avatar-row">
        <div class="avatar-preview" aria-hidden="true">
          <img v-if="user?.avatar_url" :src="user.avatar_url" alt="" />
          <span v-else>{{ avatarInitial }}</span>
        </div>
        <div>
          <button class="button" type="button" :disabled="avatarUploading" @click="avatarInput?.click()">
            {{ avatarUploading ? '上传中…' : '更换头像' }}
          </button>
          <input
            ref="avatarInput"
            class="file-input"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            :disabled="avatarUploading"
            @change="uploadAvatar"
          />
        </div>
      </div>
    </ProfileSettingsSection>

    <ProfileSettingsSection title-id="account-information-title" title="账号信息">
      <div class="settings-list">
        <div class="settings-row">
          <div class="row-copy">
            <strong>用户 ID</strong>
            <span class="mono">{{ user?.id || '-' }}</span>
          </div>
          <button class="icon-button" type="button" title="复制用户 ID" :disabled="!user?.id" @click="copyUserID">
            <Check v-if="idCopied" :size="17" />
            <Copy v-else :size="17" />
          </button>
        </div>

        <div class="settings-row">
          <div class="row-copy">
            <strong>用户名</strong>
            <span>{{ user?.username || '未设置' }}</span>
          </div>
          <button class="button" type="button" @click="openUsernameDialog">编辑</button>
        </div>

        <div class="settings-row">
          <div class="row-copy">
            <strong>邮箱</strong>
            <span>{{ user?.email || '未设置' }}</span>
          </div>
        </div>
      </div>
    </ProfileSettingsSection>

    <div v-if="usernameDialogOpen" class="dialog-mask" @click.self="closeUsernameDialog">
      <form class="dialog" @submit.prevent="saveUsername">
        <header>
          <h2>修改用户名</h2>
          <button class="icon-button" type="button" title="关闭" @click="closeUsernameDialog">
            <X :size="18" />
          </button>
        </header>
        <label>
          <span>用户名</span>
          <input ref="usernameInput" v-model="usernameDraft" maxlength="50" placeholder="未设置" />
        </label>
        <footer>
          <button class="button" type="button" @click="closeUsernameDialog">取消</button>
          <button class="button primary" type="submit" :disabled="usernameSaving || !hasUsernameChanged">保存</button>
        </footer>
      </form>
    </div>
  </div>
</template>

<script setup>
import { computed, inject, nextTick, onBeforeUnmount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Check, Copy, X } from 'lucide-vue-next'
import { userAPI } from '../../api/auth'
import ProfileSettingsSection from '../../components/profile/ProfileSettingsSection.vue'
import { PROFILE_CONTEXT_KEY } from './profileContext'

const profile = inject(PROFILE_CONTEXT_KEY)
if (!profile) throw new Error('ProfileAccount must be rendered inside Profile')

const route = useRoute()
const router = useRouter()
const { user, setUser } = profile
const usernameDraft = ref('')
const usernameSaving = ref(false)
const usernameDialogOpen = ref(false)
const usernameInput = ref(null)
const avatarInput = ref(null)
const avatarUploading = ref(false)
const idCopied = ref(false)
let copyTimer = null

const displayName = computed(() => user.value?.username || user.value?.email || 'Lite SSO 用户')
const avatarInitial = computed(() => displayName.value.slice(0, 1).toUpperCase())
const hasUsernameChanged = computed(() => usernameDraft.value.trim() !== (user.value?.username || ''))

const handleAuthenticationError = (error) => {
  if (error.status !== 401) return false
  router.replace({ path: '/login', query: { redirect: route.fullPath } })
  return true
}

const openUsernameDialog = async () => {
  usernameDraft.value = user.value?.username || ''
  usernameDialogOpen.value = true
  await nextTick()
  usernameInput.value?.focus()
}

const closeUsernameDialog = () => {
  if (usernameSaving.value) return
  usernameDraft.value = user.value?.username || ''
  usernameDialogOpen.value = false
}

const saveUsername = async () => {
  if (usernameSaving.value || !hasUsernameChanged.value) return
  usernameSaving.value = true
  try {
    const result = await userAPI.updateProfile({ username: usernameDraft.value })
    setUser(result?.data?.user)
    usernameDraft.value = user.value?.username || ''
    usernameDialogOpen.value = false
    ElMessage.success('用户名已更新')
  } catch (error) {
    if (handleAuthenticationError(error)) return
    ElMessage.error(error.message || '更新失败')
  } finally {
    usernameSaving.value = false
  }
}

const uploadAvatar = async (event) => {
  const [file] = event.target.files || []
  event.target.value = ''
  if (!file || avatarUploading.value) return
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    ElMessage.error('仅支持 JPEG、PNG 或 WebP 图片')
    return
  }
  if (file.size > 2 * 1024 * 1024) {
    ElMessage.error('头像大小不能超过 2MB')
    return
  }

  avatarUploading.value = true
  try {
    const result = await userAPI.uploadAvatar(file)
    setUser(result?.data?.user)
    ElMessage.success('头像已更新')
  } catch (error) {
    if (handleAuthenticationError(error)) return
    ElMessage.error(error.message || '头像上传失败')
  } finally {
    avatarUploading.value = false
  }
}

const copyUserID = async () => {
  if (!user.value?.id) return
  try {
    await navigator.clipboard.writeText(user.value.id)
    idCopied.value = true
    ElMessage.success('用户 ID 已复制')
    window.clearTimeout(copyTimer)
    copyTimer = window.setTimeout(() => { idCopied.value = false }, 1400)
  } catch (error) {
    ElMessage.error(error.message || '复制失败')
  }
}

onBeforeUnmount(() => window.clearTimeout(copyTimer))
</script>

<style scoped>
.settings-view {
  display: grid;
  gap: 30px;
}

.avatar-row {
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 16px;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
}

.avatar-preview {
  display: flex;
  width: 64px;
  height: 64px;
  flex: 0 0 64px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid var(--profile-border-muted);
  border-radius: 50%;
  background: var(--profile-surface-subtle);
  color: var(--profile-text-muted);
  font-size: 22px;
  font-weight: 600;
}

.avatar-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.file-input {
  display: none;
}

.settings-list {
  overflow: hidden;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
}

.settings-row {
  display: flex;
  min-height: 62px;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 12px 16px;
  border-top: 1px solid var(--profile-divider);
}

.settings-row:first-child {
  border-top: 0;
}

.row-copy {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.row-copy strong {
  font-size: 14px;
}

.row-copy span {
  overflow: hidden;
  color: var(--profile-text-muted);
  font-size: 13px;
  text-overflow: ellipsis;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
}

.button,
.icon-button {
  min-height: 32px;
  border: 1px solid var(--profile-border-muted);
  border-radius: 6px;
  background: var(--profile-surface-subtle);
  color: var(--profile-text-strong);
  cursor: pointer;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
}

.button {
  padding: 5px 12px;
}

.button.primary {
  border-color: var(--profile-accent);
  background: var(--profile-accent);
  color: #ffffff;
}

.button:hover:not(:disabled),
.icon-button:hover:not(:disabled) {
  background: var(--profile-surface-hover);
}

.button.primary:hover:not(:disabled) {
  background: var(--profile-accent-hover);
}

.button:disabled,
.icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.icon-button {
  display: inline-flex;
  width: 32px;
  flex: 0 0 32px;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.dialog-mask {
  position: fixed;
  z-index: 30;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 18px;
  background: var(--profile-overlay);
}

.dialog {
  display: grid;
  width: min(420px, 100%);
  box-sizing: border-box;
  gap: 18px;
  border: 1px solid var(--profile-border);
  border-radius: 8px;
  background: var(--profile-surface-subtle);
  box-shadow: var(--profile-shadow);
  padding: 20px;
}

.dialog header,
.dialog footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dialog h2 {
  margin: 0;
  font-size: 18px;
}

.dialog label {
  display: grid;
  gap: 8px;
  color: var(--profile-text-strong);
  font-size: 14px;
  font-weight: 600;
}

.dialog input {
  height: 34px;
  box-sizing: border-box;
  border: 1px solid var(--profile-text-faint);
  border-radius: 6px;
  color: var(--profile-text-strong);
  font: inherit;
  padding: 5px 10px;
}

.dialog input:focus {
  border-color: var(--profile-accent);
  box-shadow: 0 0 0 3px var(--profile-focus-ring);
  outline: none;
}

.dialog footer {
  justify-content: flex-end;
}

@media (max-width: 560px) {
  .settings-row {
    align-items: flex-start;
  }
}
</style>

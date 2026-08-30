<template>
  <div class="appearance-view">
    <ProfileSettingsSection
      title-id="theme-preference-title"
      title="主题偏好"
      description="选择您喜欢的展示方式。可以使用固定主题，也可以跟随系统在亮色与暗色主题间自动切换。所选设置会立即生效并自动保存。"
    >
      <div class="theme-control">
        <label for="theme-mode">主题模式</label>
        <div class="theme-mode-row">
          <select id="theme-mode" :value="profileThemePreference" @change="changeTheme">
            <option value="light">亮色主题</option>
            <option value="dark">暗色主题</option>
            <option value="system">跟随系统</option>
          </select>
          <p>{{ modeDescription }}</p>
        </div>
      </div>

      <div class="theme-previews" aria-label="主题预览">
        <article class="theme-preview" :class="{ active: effectiveProfileTheme === 'light' }">
          <header class="preview-header">
            <strong><Sun :size="18" aria-hidden="true" />亮色主题</strong>
            <span v-if="effectiveProfileTheme === 'light'" class="active-badge">
              <Check :size="13" aria-hidden="true" />
              当前主题
            </span>
          </header>
          <div class="preview-body">
            <p>当设备使用亮色外观时，此主题会生效。</p>
            <div class="preview-image">
              <img :src="lightPreview" alt="亮色主题界面预览" />
            </div>
          </div>
          <footer>亮色默认</footer>
        </article>

        <article class="theme-preview" :class="{ active: effectiveProfileTheme === 'dark' }">
          <header class="preview-header">
            <strong><Moon :size="18" aria-hidden="true" />暗色主题</strong>
            <span v-if="effectiveProfileTheme === 'dark'" class="active-badge">
              <Check :size="13" aria-hidden="true" />
              当前主题
            </span>
          </header>
          <div class="preview-body">
            <p>当设备使用暗色外观时，此主题会生效。</p>
            <div class="preview-image">
              <img :src="darkPreview" alt="暗色主题界面预览" />
            </div>
          </div>
          <footer>暗色默认</footer>
        </article>
      </div>
    </ProfileSettingsSection>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Check, Moon, Sun } from 'lucide-vue-next'
import ProfileSettingsSection from '../../components/profile/ProfileSettingsSection.vue'
import darkPreview from '../../assets/profile-theme/dark.png'
import lightPreview from '../../assets/profile-theme/light.png'
import {
  effectiveProfileTheme,
  profileThemePreference,
  setProfileThemePreference
} from '../../utils/profileTheme'

const modeDescriptions = {
  light: '系统将始终使用亮色主题',
  dark: '系统将始终使用暗色主题',
  system: '主题将匹配当前系统的外观设置'
}

const modeDescription = computed(() => modeDescriptions[profileThemePreference.value] || modeDescriptions.system)

const changeTheme = (event) => {
  setProfileThemePreference(event.target.value)
}
</script>

<style scoped>
.appearance-view {
  display: grid;
  gap: 22px;
}

.theme-control {
  display: grid;
  gap: 8px;
}

.theme-control label {
  color: var(--profile-text-strong);
  font-size: 14px;
  font-weight: 600;
}

.theme-mode-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.theme-control select {
  width: min(260px, 100%);
  flex: 0 0 min(260px, 100%);
  min-height: 36px;
  box-sizing: border-box;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
  background: var(--profile-surface);
  color: var(--profile-text);
  font: inherit;
  font-size: 14px;
  padding: 6px 34px 6px 10px;
}

.theme-control select:focus {
  border-color: var(--profile-accent);
  box-shadow: 0 0 0 3px var(--profile-focus-ring);
  outline: none;
}

.theme-control p {
  margin: 0;
  color: var(--profile-text-muted);
  font-size: 13px;
  line-height: 1.5;
}

.theme-previews {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.theme-preview {
  overflow: hidden;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
  background: var(--profile-surface);
}

.theme-preview.active {
  border-color: var(--profile-accent);
  box-shadow: 0 0 0 1px var(--profile-accent);
}

.theme-preview.active .preview-header {
  background: var(--profile-accent-soft);
}

.preview-header {
  display: flex;
  min-height: 54px;
  box-sizing: border-box;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--profile-border);
  background: var(--profile-surface-subtle);
}

.preview-header strong {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--profile-text-strong);
  font-size: 15px;
}

.active-badge {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 4px;
  border: 1px solid var(--profile-accent);
  border-radius: 999px;
  background: transparent;
  color: var(--profile-accent);
  font-size: 11px;
  font-weight: 600;
  padding: 3px 7px;
}

.preview-body {
  display: grid;
  gap: 14px;
  padding: 16px;
}

.preview-body p {
  margin: 0;
  color: var(--profile-text-muted);
  font-size: 13px;
  line-height: 1.5;
}

.preview-image {
  padding: 14px;
  border: 1px solid var(--profile-border);
  border-radius: 6px;
  background: var(--profile-surface-subtle);
}

.preview-image img {
  display: block;
  width: 100%;
  height: auto;
  margin: 0 auto;
}

.theme-preview footer {
  padding: 12px 16px;
  border-top: 1px solid var(--profile-border);
  color: var(--profile-text-strong);
  font-size: 14px;
  font-weight: 600;
}

@media (max-width: 620px) {
  .theme-mode-row {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }

  .theme-control select {
    width: 100%;
    flex-basis: auto;
  }

  .theme-previews {
    grid-template-columns: 1fr;
  }
}
</style>

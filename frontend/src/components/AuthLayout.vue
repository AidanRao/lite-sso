<template>
  <div class="auth-page" :class="{ 'auth-page--fixed': fixedMobile }" :style="viewportStyle">
    <AuthSplashPane class="auth-splash" title="身份认证系统" :target-name="targetName" :show-docs-link="showDocsLink" />
    <main class="auth-pane">
      <section class="auth-card auth-theme" :class="{ 'auth-card--plain': !showLogo }" :aria-label="title">
        <slot name="corner" />
        <header class="auth-header" :class="{ 'auth-header--corner': $slots.corner }">
          <ApplicationLogo v-if="showLogo" :src="logo" :label="targetName" size="large" class="auth-logo" />
          <h1>{{ title }}</h1>
        </header>
        <slot />
        <footer v-if="$slots.footer" class="auth-footer"><slot name="footer" /></footer>
      </section>
    </main>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import ApplicationLogo from './ApplicationLogo.vue'
import AuthSplashPane from './AuthSplashPane.vue'
import '../auth.css'

const props = defineProps({
  title: { type: String, required: true },
  targetName: { type: String, default: '账号中心' },
  logo: { type: String, default: '' },
  showLogo: { type: Boolean, default: false },
  showDocsLink: { type: Boolean, default: false },
  fixedMobile: { type: Boolean, default: false }
})

const viewportStyle = ref({})
const syncViewport = () => {
  const viewport = window.visualViewport
  if (!viewport || viewport.scale !== 1) return
  viewportStyle.value = {
    '--auth-viewport-height': `${viewport.height}px`,
    '--auth-viewport-top': `${viewport.offsetTop}px`
  }
}
onMounted(() => {
  if (!props.fixedMobile) return
  syncViewport()
  window.visualViewport?.addEventListener('resize', syncViewport)
  window.visualViewport?.addEventListener('scroll', syncViewport)
})
onUnmounted(() => {
  window.visualViewport?.removeEventListener('resize', syncViewport)
  window.visualViewport?.removeEventListener('scroll', syncViewport)
})
</script>

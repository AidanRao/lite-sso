<template>
  <div class="auth-page">
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
import ApplicationLogo from './ApplicationLogo.vue'
import AuthSplashPane from './AuthSplashPane.vue'
import '../auth.css'

defineProps({
  title: { type: String, required: true },
  targetName: { type: String, default: '账号中心' },
  logo: { type: String, default: '' },
  showLogo: { type: Boolean, default: false },
  showDocsLink: { type: Boolean, default: false }
})
</script>

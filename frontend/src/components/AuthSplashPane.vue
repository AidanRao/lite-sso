<template>
  <div
    class="auth-splash-pane flex-col items-center justify-center p-8 xl:p-12 relative overflow-hidden"
    :style="{ backgroundImage: splashBackgroundImage }"
  >
    <div class="relative z-10 text-center">
      <h1 class="auth-splash-title text-5xl font-bold mb-4 tracking-tight">{{ title }}</h1>
      <p class="auth-splash-subtitle text-lg max-w-md leading-relaxed">
        <slot name="subtitle">
          正在前往
          <span class="font-semibold">{{ targetName }}</span>
        </slot>
      </p>
      <div class="mt-12 flex justify-center">
        <img
          :src="imageSrc"
          :alt="imageAlt"
          class="auth-splash-image w-72 h-72 rounded-2xl object-cover transition-all duration-500"
        />
      </div>
    </div>

    <router-link
      v-if="showDocsLink"
      to="/docs"
      class="auth-splash-doc-link absolute bottom-8 left-1/2 z-10 -translate-x-1/2 text-sm font-medium transition-colors"
    >
      开发文档
    </router-link>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'

defineProps({
  title: {
    type: String,
    default: '身份认证系统'
  },
  targetName: {
    type: String,
    default: 'Lite SSO'
  },
  imageSrc: {
    type: String,
    default: '/assets/images/default.png'
  },
  imageAlt: {
    type: String,
    default: 'SSO Illustration'
  },
  showDocsLink: {
    type: Boolean,
    default: true
  }
})

const splashImages = [
  '/assets/splash/heiHaiAn_2-0333aeb7.jpg',
  '/assets/splash/septimont_9-1c57dd39.webp'
]

const currentSplashIndex = ref(0)
const splashInterval = ref(null)

const currentSplashImage = computed(() => splashImages[currentSplashIndex.value] || splashImages[0])
const splashBackgroundImage = computed(() => (
  `linear-gradient(135deg, rgba(82, 54, 34, 0.5), rgba(140, 98, 52, 0.34)), linear-gradient(180deg, rgba(25, 22, 18, 0.18), rgba(25, 22, 18, 0.42)), url("${currentSplashImage.value}")`
))

onMounted(() => {
  if (splashImages.length > 1) {
    splashInterval.value = setInterval(() => {
      currentSplashIndex.value = (currentSplashIndex.value + 1) % splashImages.length
    }, 30000)
  }
})

onUnmounted(() => {
  if (splashInterval.value) {
    clearInterval(splashInterval.value)
  }
})
</script>

<style scoped>
.auth-splash-pane {
  background-color: #eee8d7;
  background-position: center;
  background-size: cover;
  color: #fff8ec;
}

.auth-splash-title {
  text-shadow:
    0 2px 8px rgba(25, 18, 12, 0.5),
    0 10px 28px rgba(25, 18, 12, 0.32);
}

.auth-splash-subtitle {
  color: rgba(255, 248, 236, 0.92);
  text-shadow: 0 2px 10px rgba(25, 18, 12, 0.48);
}

.auth-splash-image {
  border: 2px solid rgba(255, 255, 255, 0.72);
  box-shadow: 0 20px 44px rgba(25, 22, 18, 0.2);
}

.auth-splash-image:hover {
  transform: scale(1.03);
}

.auth-splash-doc-link {
  color: rgba(255, 248, 236, 0.72);
  text-shadow: 0 2px 8px rgba(25, 18, 12, 0.42);
}

.auth-splash-doc-link:hover {
  color: #fff8ec;
}
</style>

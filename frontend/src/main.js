import { createApp } from 'vue'
import router from './router'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './style.css'
import './profile-theme.css'
import App from './App.vue'
import { refreshAccessToken } from './api/auth'
import { initializeProfileTheme } from './utils/profileTheme'

const app = createApp(App)
app.use(router)
app.use(ElementPlus)
initializeProfileTheme(router)
refreshAccessToken().catch(() => {}).finally(() => {
  app.mount('#app')
})

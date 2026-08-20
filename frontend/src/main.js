import { createApp } from 'vue'
import router from './router'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './style.css'
import App from './App.vue'
import { refreshAccessToken } from './api/auth'

const app = createApp(App)
app.use(router)
app.use(ElementPlus)
refreshAccessToken().catch(() => {}).finally(() => {
  app.mount('#app')
})

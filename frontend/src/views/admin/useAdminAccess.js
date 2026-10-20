import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

export function useAdminAccess() {
  const route = useRoute()
  const router = useRouter()
  const handleAccessError = (error) => {
    if (error.status === 401) {
      router.replace({ path: '/login', query: { redirect: route.fullPath } })
      return true
    }
    if (error.status === 403) {
      ElMessage.error('无管理员权限')
      router.replace('/profile')
      return true
    }
    return false
  }
  return { handleAccessError }
}

import { createRouter, createWebHistory } from 'vue-router'
import Login from '../views/Login.vue'
import Register from '../views/Register.vue'
import ResetPassword from '../views/ResetPassword.vue'
import OAuthCallback from '../views/OAuthCallback.vue'
import Profile from '../views/Profile.vue'

const routes = [
  {
    path: '/',
    redirect: '/profile'
  },
  {
    path: '/login',
    name: 'Login',
    meta: { title: '登录' },
    component: Login
  },
  {
    path: '/logout',
    name: 'Logout',
    meta: { title: '退出登录' },
    component: () => import('../views/Logout.vue')
  },
  {
    path: '/register',
    name: 'Register',
    meta: { title: '创建账号' },
    component: Register
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    meta: { title: '找回密码' },
    component: ResetPassword
  },
  {
    path: '/oauth/callback',
    name: 'OAuthCallback',
    meta: { title: '登录回调' },
    component: OAuthCallback
  },
  {
    path: '/profile/third-party-bind',
    name: 'ThirdPartyBindPreview',
    meta: { title: '绑定第三方账号' },
    component: () => import('../views/ThirdPartyBindPreview.vue')
  },
  {
    path: '/profile',
    component: Profile,
    redirect: '/profile/account',
    children: [
      {
        path: 'archived/audit-logs',
        name: 'ProfileAuditLogs',
        meta: { title: '审计日志', featureKey: 'profile.audit_logs' },
        component: () => import('../views/profile/ProfileAuditLogs.vue')
      },
      {
        path: 'account',
        name: 'ProfileAccount',
        meta: { title: '账号信息' },
        component: () => import('../views/profile/ProfileAccount.vue')
      },
      {
        path: 'appearance',
        name: 'ProfileAppearance',
        meta: { title: '外观设置' },
        component: () => import('../views/profile/ProfileAppearance.vue')
      },
      {
        path: 'access/authentication',
        name: 'ProfileAuthentication',
        meta: { title: '登录方式' },
        component: () => import('../views/profile/ProfileAuthentication.vue')
      },
      {
        path: 'access/emails',
        name: 'ProfileEmails',
        meta: { title: '邮箱管理' },
        component: () => import('../views/profile/ProfileEmails.vue')
      },
      {
        path: 'access/emails/verify',
        name: 'ProfileEmailVerification',
        meta: { title: '验证邮箱' },
        component: () => import('../views/profile/ProfileEmails.vue')
      },
      {
        path: 'access/sessions',
        name: 'ProfileSessions',
        meta: { title: '登录设备' },
        component: () => import('../views/profile/ProfileSessions.vue')
      },
      {
        path: 'integrations/applications',
        name: 'ProfileApplications',
        meta: { title: '已登录应用' },
        component: () => import('../views/profile/ProfileApplications.vue')
      }
    ]
  },
  {
    path: '/admin',
    name: 'Admin',
    meta: { title: '系统管理' },
    component: () => import('../views/Admin.vue')
  },
  {
    path: '/docs/:slug?',
    name: 'Docs',
    meta: { title: '开发文档' },
    component: () => import('../views/Docs.vue')
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.afterEach(async (to, from, failure) => {
  if (failure) return

  document.title = to.meta.title ? `${to.meta.title} - 统一身份认证` : '统一身份认证'

  if (to.name === 'Docs') {
    const { documents, findDocument } = await import('../docs')
    if (router.currentRoute.value !== to) return

    const article = to.params.slug ? findDocument(String(to.params.slug)) : documents[0]
    document.title = `${article?.title || '未找到文档'} - 统一身份认证`
  }
})

export default router

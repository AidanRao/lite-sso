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
    component: Login
  },
  {
    path: '/register',
    name: 'Register',
    component: Register
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    component: ResetPassword
  },
  {
    path: '/oauth/callback',
    name: 'OAuthCallback',
    component: OAuthCallback
  },
  {
    path: '/profile/third-party-bind',
    name: 'ThirdPartyBindPreview',
    component: () => import('../views/ThirdPartyBindPreview.vue')
  },
  {
    path: '/profile',
    component: Profile,
    redirect: '/profile/account',
    children: [
      {
        path: 'account',
        name: 'ProfileAccount',
        component: () => import('../views/profile/ProfileAccount.vue')
      },
      {
        path: 'access/authentication',
        name: 'ProfileAuthentication',
        component: () => import('../views/profile/ProfileAuthentication.vue')
      },
      {
        path: 'access/sessions',
        name: 'ProfileSessions',
        component: () => import('../views/profile/ProfileSessions.vue')
      },
      {
        path: 'integrations/applications',
        name: 'ProfileApplications',
        component: () => import('../views/profile/ProfileApplications.vue')
      }
    ]
  },
  {
    path: '/admin',
    name: 'Admin',
    component: () => import('../views/Admin.vue')
  },
  {
    path: '/docs/:slug?',
    name: 'Docs',
    component: () => import('../views/Docs.vue')
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router

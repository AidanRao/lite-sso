import { readonly, ref } from 'vue'

export const PROFILE_THEME_COOKIE = 'lite_sso_profile_theme'
export const PROFILE_THEME_DEFAULT = 'system'
export const PROFILE_THEME_VALUES = ['light', 'dark', 'system']

const preference = ref(PROFILE_THEME_DEFAULT)
const effectiveTheme = ref('light')

let activeProfilePath = false
let darkModeQuery = null
let initialized = false

export const profileThemePreference = readonly(preference)
export const effectiveProfileTheme = readonly(effectiveTheme)

export const isProfilePath = (path) => path === '/profile' || path.startsWith('/profile/')

export const normalizeProfileThemePreference = (value) => (
  PROFILE_THEME_VALUES.includes(value) ? value : PROFILE_THEME_DEFAULT
)

export const readProfileThemePreference = (cookieString = '') => {
  const prefix = `${PROFILE_THEME_COOKIE}=`
  const cookie = cookieString
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(prefix))

  if (!cookie) return PROFILE_THEME_DEFAULT

  try {
    return normalizeProfileThemePreference(decodeURIComponent(cookie.slice(prefix.length)))
  } catch {
    return PROFILE_THEME_DEFAULT
  }
}

export const serializeProfileThemeCookie = (value, { secure = false } = {}) => {
  const normalized = normalizeProfileThemePreference(value)
  const attributes = [
    `${PROFILE_THEME_COOKIE}=${encodeURIComponent(normalized)}`,
    'Path=/profile',
    'Max-Age=31536000',
    'SameSite=Lax'
  ]
  if (secure) attributes.push('Secure')
  return attributes.join('; ')
}

export const resolveProfileTheme = (value, systemPrefersDark = false) => {
  const normalized = normalizeProfileThemePreference(value)
  if (normalized === 'system') return systemPrefersDark ? 'dark' : 'light'
  return normalized
}

const applyEffectiveTheme = () => {
  const prefersDark = darkModeQuery?.matches || false
  effectiveTheme.value = resolveProfileTheme(preference.value, prefersDark)

  if (activeProfilePath) {
    document.documentElement.dataset.profileTheme = effectiveTheme.value
  }
}

const handleSystemThemeChange = () => {
  if (preference.value === 'system') applyEffectiveTheme()
}

const startSystemThemeListener = () => {
  if (darkModeQuery || typeof window.matchMedia !== 'function') return
  darkModeQuery = window.matchMedia('(prefers-color-scheme: dark)')
  darkModeQuery.addEventListener?.('change', handleSystemThemeChange)
}

const stopSystemThemeListener = () => {
  if (!darkModeQuery) return
  darkModeQuery.removeEventListener?.('change', handleSystemThemeChange)
  darkModeQuery = null
}

const applyProfileThemeForPath = (path) => {
  activeProfilePath = isProfilePath(path)
  if (!activeProfilePath) {
    stopSystemThemeListener()
    delete document.documentElement.dataset.profileTheme
    return
  }

  preference.value = readProfileThemePreference(document.cookie)
  startSystemThemeListener()
  applyEffectiveTheme()
}

export const setProfileThemePreference = (value) => {
  preference.value = normalizeProfileThemePreference(value)
  document.cookie = serializeProfileThemeCookie(preference.value, {
    secure: window.location.protocol === 'https:'
  })
  applyEffectiveTheme()
}

export const initializeProfileTheme = (router) => {
  if (initialized) return
  initialized = true
  applyProfileThemeForPath(window.location.pathname)
  router.afterEach((to) => applyProfileThemeForPath(to.path))
}

export const EMAIL_SETUP_PATH = '/setup-email'

// Continuation URLs are supplied by the login flow, but remain editable in the address bar.
export const getEmailSetupRedirect = (value) => {
  if (typeof value !== 'string' || /[\\\r\n\t]/.test(value)) return '/profile'
  const target = value.trim()
  if (!target.startsWith('/') || target.startsWith('//')) return '/profile'
  try {
    const parsed = new URL(target, 'https://sso.invalid')
    if (parsed.origin !== 'https://sso.invalid' || parsed.pathname.replace(/\/+$/, '') === EMAIL_SETUP_PATH) return '/profile'
    return target
  } catch {
    return '/profile'
  }
}

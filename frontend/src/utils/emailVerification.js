const pendingTokenKey = 'sso.pending-email-verification'

export const getEmailVerificationToken = (hash = '') => {
  const value = String(hash).replace(/^#/, '')
  if (!value) return ''
  return new URLSearchParams(value).get('token')?.trim() || ''
}

// Keep the fragment out of login redirect queries and OAuth state across reauthentication.
export const rememberEmailVerificationToken = (token, storage = window.sessionStorage) => {
  storage.setItem(pendingTokenKey, token)
}

export const readEmailVerificationToken = (storage) => {
  try { return (storage || window.sessionStorage).getItem(pendingTokenKey) || '' }
  catch { return '' }
}

export const forgetEmailVerificationToken = (storage) => {
  try { (storage || window.sessionStorage).removeItem(pendingTokenKey) }
  catch { /* Browser storage may be unavailable. */ }
}

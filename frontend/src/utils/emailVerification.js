export const getEmailVerificationToken = (hash = '') => {
  const value = String(hash).replace(/^#/, '')
  if (!value) return ''
  return new URLSearchParams(value).get('token')?.trim() || ''
}

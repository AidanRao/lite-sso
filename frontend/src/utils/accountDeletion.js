const summaryKey = 'account-deletion-summary'
export const deletionPhrase = '注销我的账号'

export const deletionIdentifiers = (user, emails = []) => {
  const identifiers = []
  if (user?.username) identifiers.push({ value: user.username, email: false })
  for (const email of emails) {
    if (email.verified) identifiers.push({ value: email.email, email: true })
  }
  if (!identifiers.length && user?.id) identifiers.push({ value: user.id, email: false })
  return identifiers
}

export const deletionConfirmationMatches = (identity, phrase, identifiers) => {
  if (phrase !== deletionPhrase) return false
  const value = identity.trim()
  return identifiers.some(identifier => identifier.email
    ? identifier.value.toLowerCase() === value.toLowerCase()
    : identifier.value === value)
}

export const formatDeletionTime = (value) => value ? new Intl.DateTimeFormat('zh-CN', {
  timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false
}).format(new Date(value)) : ''

export const rememberDeletion = (data) => {
  try { sessionStorage.setItem(summaryKey, JSON.stringify({ deletion_scheduled_at: data.deletion_scheduled_at })) } catch { /* Storage may be unavailable. */ }
}
export const readDeletionSummary = () => {
  try {
    const value = JSON.parse(sessionStorage.getItem(summaryKey) || 'null')
    return value?.deletion_scheduled_at && Number.isFinite(Date.parse(value.deletion_scheduled_at)) ? value : null
  } catch { return null }
}
export const forgetDeletion = () => {
  try { sessionStorage.removeItem(summaryKey) } catch { /* Storage may be unavailable. */ }
}

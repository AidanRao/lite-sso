const maximumVariablesBytes = 64 * 1024

export function parseMessageSettings(senderKey, templateKey, text) {
  const sender_key = senderKey.trim()
  const template_key = templateKey.trim()
  if (!sender_key || !template_key || [...sender_key].length > 100 || [...template_key].length > 100) {
    throw new Error('sender-key 和 template-key 必填，且各不超过 100 个字符。')
  }
  let variables
  try { variables = JSON.parse(text) } catch { throw new Error('参数必须是有效的 JSON 对象。') }
  if (!variables || Array.isArray(variables) || typeof variables !== 'object') throw new Error('参数必须是 JSON 对象，例如 {"title":"通知"}。')
  if (new TextEncoder().encode(JSON.stringify(variables)).length > maximumVariablesBytes) throw new Error('参数不能超过 64 KiB。')
  return { sender_key, template_key, variables }
}

export function setMessageSelection(selection, users, selected) {
  for (const user of users) {
    if (!user.email?.trim()) continue
    if (selected) selection.set(user.user_id, { ...user })
    else selection.delete(user.user_id)
  }
}

export function createMessageRun(users, settings, newKey = () => crypto.randomUUID()) {
  return {
    settings: JSON.parse(JSON.stringify(settings)),
    entries: users.map(user => ({ ...user, key: newKey(), status: 'pending', error: '' }))
  }
}

// Each attempt reuses its original snapshot and key; only a new run creates new keys.
export async function sendMessageRun(run, send, { shouldStop = () => false, onAccessError = () => {} } = {}) {
  const pending = run.entries.filter(entry => entry.status !== 'accepted')
  let cursor = 0
  let accessLost = false
  const worker = async () => {
    while (!accessLost && !shouldStop() && cursor < pending.length) {
      const entry = pending[cursor++]
      entry.status = 'sending'
      entry.error = ''
      try {
        await send(entry, run.settings)
        entry.status = 'accepted'
      } catch (error) {
        entry.status = !error.status || error.data?.code === 'MESSAGE_SUBMISSION_UNKNOWN' || error.status === 504 ? 'unknown' : 'failed'
        entry.error = error.message || '提交失败'
        if (!accessLost && (error.status === 401 || error.status === 403)) {
          accessLost = true
          onAccessError(error)
        }
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(3, pending.length) }, worker))
}

import { reactive, readonly } from 'vue'

const state = reactive({
  visible: false,
  descriptor: null,
  requestID: 0
})

let cachedGrant = null
let pendingPromise = null
let resolvePending = null
let rejectPending = null

export class ReauthCancelledError extends Error {
  constructor() {
    super('重新验证已取消')
    this.name = 'ReauthCancelledError'
  }
}

export const reauthState = readonly(state)

export const isReauthCancelled = (error) => error instanceof ReauthCancelledError

export const getCachedReauthToken = () => {
  if (!cachedGrant || Date.now() >= cachedGrant.expiresAt) {
    cachedGrant = null
    return ''
  }
  return cachedGrant.token
}

export const requestReauth = (descriptor) => {
  const token = getCachedReauthToken()
  if (token) return Promise.resolve(token)
  if (pendingPromise) return pendingPromise

  state.descriptor = normalizeDescriptor(descriptor)
  state.requestID += 1
  state.visible = true
  pendingPromise = new Promise((resolve, reject) => {
    resolvePending = resolve
    rejectPending = reject
  })
  return pendingPromise
}

export const completeReauth = (grant) => {
  const token = String(grant?.token || '')
  if (!token || !resolvePending) return
  cachedGrant = {
    token,
    expiresAt: Date.now() + Math.max(0, Number(grant?.expires_in || 0) - 2) * 1000
  }
  const resolve = resolvePending
  resetPending()
  resolve(token)
}

export const cancelReauth = () => {
  if (!rejectPending) return
  const reject = rejectPending
  resetPending()
  reject(new ReauthCancelledError())
}

export const clearReauth = () => {
  cachedGrant = null
  if (rejectPending) {
    const reject = rejectPending
    resetPending()
    reject(new ReauthCancelledError())
    return
  }
  state.visible = false
  state.descriptor = null
}

export const invalidateReauthToken = () => {
  cachedGrant = null
}

const resetPending = () => {
  state.visible = false
  state.descriptor = null
  pendingPromise = null
  resolvePending = null
  rejectPending = null
}

const normalizeDescriptor = (descriptor) => {
  const methods = Array.isArray(descriptor?.methods)
    ? descriptor.methods.filter((method) => method === 'passkey' || method === 'email')
    : []
  return {
    methods: [...new Set(methods)],
    max_age: Number(descriptor?.max_age || 0),
    email_hint: String(descriptor?.email_hint || ''),
    username: String(descriptor?.username || ''),
    avatar_url: String(descriptor?.avatar_url || '')
  }
}

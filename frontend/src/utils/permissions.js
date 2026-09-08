import { reactive, readonly } from 'vue'

// Each store belongs to one document. Session resets invalidate all older responses.
export function createPermissionStore(fetchPermissions) {
  const state = reactive({ isAdmin: false, features: {}, loaded: false, loading: false, error: null })
  let version = 0
  let session = 0
  let pending = null
  let denialRefresh = null
  let deniedKeys = new Set()

  function reset() {
    version++
    session++
    pending = null
    denialRefresh = null
    deniedKeys = new Set()
    Object.assign(state, { isAdmin: false, features: {}, loaded: false, loading: false, error: null })
  }

  function load() {
    if (pending) return pending
    if (state.loaded) return Promise.resolve()
    const current = version
    state.loading = true
    state.error = null
    const request = Promise.resolve().then(fetchPermissions).then(result => {
      if (current !== version) return
      state.isAdmin = result.data.is_admin === true
      state.features = { ...result.data.features }
      for (const key of deniedKeys) {
        state.features[key] = { enabled: false, stage: state.features[key]?.stage || 'beta' }
      }
      state.loaded = true
    }).catch(error => {
      if (current !== version) return
      state.error = error
      throw error
    }).finally(() => {
      if (pending === request) { pending = null; state.loading = false }
    })
    pending = request
    return request
  }

  function refresh({ keepDenials = false } = {}) {
    if (!keepDenials) { deniedKeys = new Set(); denialRefresh = null }
    version++
    pending = null
    state.loaded = false
    return load()
  }

  function deny(key) {
    if (typeof key !== 'string' || !key) return Promise.resolve()
    deniedKeys.add(key)
    state.features = { ...state.features, [key]: { enabled: false, stage: state.features[key]?.stage || 'beta' } }
    if (denialRefresh) return denialRefresh
    // Invalidate any read started before the denial, then coalesce new denials.
    const request = refresh({ keepDenials: true }).finally(() => {
      if (denialRefresh === request) denialRefresh = null
    })
    denialRefresh = request
    return request
  }

  return { state: readonly(state), load, refresh, reset, deny, sessionVersion: () => session }
}

export const permissions = createPermissionStore(async () => {
  const { userAPI } = await import('../api/auth.js')
  return userAPI.getPermissions()
})

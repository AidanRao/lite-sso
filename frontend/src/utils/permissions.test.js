import assert from 'node:assert/strict'
import test from 'node:test'
import { createPermissionStore } from './permissions.js'

const result = enabled => ({ data: { is_admin: true, features: { logs: { enabled, stage: 'beta' } } } })
const deferred = () => {
  let resolve, reject
  const promise = new Promise((a, b) => { resolve = a; reject = b })
  return { promise, resolve, reject }
}

test('concurrent consumers and later page mounts reuse one permission request', async () => {
  const request = deferred()
  let calls = 0
  const store = createPermissionStore(() => { calls++; return request.promise })
  const first = store.load()
  assert.equal(store.load(), first)
  assert.equal(store.state.loaded, false)
  assert.equal(store.state.isAdmin, false)
  request.resolve(result(true))
  await first
  await store.load()
  await store.load()
  assert.equal(calls, 1)
  assert.equal(store.state.features.logs.enabled, true)
})

test('failure remains retryable without implicit request loops', async () => {
  let calls = 0
  const store = createPermissionStore(() => { if (++calls === 1) throw new Error('offline'); return result(true) })
  await assert.rejects(store.load(), /offline/)
  assert.equal(store.state.loading, false)
  assert.equal(store.state.loaded, false)
  assert.equal(calls, 1)
  await store.load()
  assert.equal(store.state.error, null)
  assert.equal(calls, 2)
})

test('session reset ignores late success and late failure from old sessions', async () => {
  for (const reject of [false, true]) {
    const old = deferred()
    let calls = 0
    const store = createPermissionStore(() => ++calls === 1 ? old.promise : result(false))
    const pending = store.load()
    await Promise.resolve()
    store.reset()
    await store.load()
    if (reject) old.reject(new Error('old session failed'))
    else old.resolve(result(true))
    await pending
    assert.equal(store.state.features.logs.enabled, false)
    assert.equal(store.state.error, null)
    assert.equal(store.state.loaded, true)
  }
})

test('denial is immediate, invalidates pre-denial reads and coalesces refreshes', async () => {
  const old = deferred(), current = deferred()
  let calls = 0
  const store = createPermissionStore(() => ++calls === 1 ? old.promise : current.promise)
  const initial = store.load()
  await Promise.resolve()
  const denied = store.deny('logs')
  assert.equal(store.state.features.logs.enabled, false)
  assert.equal(store.deny('another'), denied)
  old.resolve(result(true))
  await initial
  assert.equal(store.state.features.logs.enabled, false)
  current.resolve(result(true))
  await denied
  assert.equal(calls, 2)
  assert.equal(store.state.features.logs.enabled, false)
  assert.equal(store.state.features.another.enabled, false)
  await store.refresh()
  assert.equal(store.state.features.logs.enabled, true)
})

test('explicit refresh after saving does not reuse a cached decision', async () => {
  let calls = 0
  const store = createPermissionStore(() => result(++calls === 1))
  await store.load()
  await store.refresh()
  assert.equal(calls, 2)
  assert.equal(store.state.features.logs.enabled, false)
})

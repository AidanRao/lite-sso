import assert from 'node:assert/strict'
import test from 'node:test'

import {
  cancelReauth,
  clearReauth,
  completeReauth,
  getCachedReauthToken,
  invalidateReauthToken,
  isReauthCancelled,
  reauthState,
  requestReauth
} from './reauthCoordinator.js'

test('requestReauth shares one prompt and caches the completed grant', async () => {
  clearReauth()
  const descriptor = { methods: ['passkey', 'email', 'unknown'], max_age: 300, email_hint: 'o***@example.com' }
  const first = requestReauth(descriptor)
  const second = requestReauth(descriptor)

  assert.equal(first, second)
  assert.equal(reauthState.visible, true)
  assert.deepEqual(reauthState.descriptor.methods, ['passkey', 'email'])

  completeReauth({ token: 'grant-token', expires_in: 300 })
  assert.deepEqual(await Promise.all([first, second]), ['grant-token', 'grant-token'])
  assert.equal(getCachedReauthToken(), 'grant-token')
  assert.equal(reauthState.visible, false)

  invalidateReauthToken()
  assert.equal(getCachedReauthToken(), '')
})

test('cancelReauth rejects the pending operation without caching a token', async () => {
  clearReauth()
  const pending = requestReauth({ methods: ['email'], max_age: 300 })
  cancelReauth()

  await assert.rejects(pending, error => isReauthCancelled(error))
  assert.equal(getCachedReauthToken(), '')
  assert.equal(reauthState.visible, false)
})

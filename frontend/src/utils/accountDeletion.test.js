import assert from 'node:assert/strict'
import test from 'node:test'
import { AxiosError } from 'axios'
import { deletionIdentifiers, deletionConfirmationMatches, formatDeletionTime } from './accountDeletion.js'
import { api, userAPI } from '../api/auth.js'
import { cancelReauth, clearReauth, completeReauth, reauthState } from './reauthCoordinator.js'

test('confirmation accepts only current identity and exact phrase', () => {
  const identifiers = deletionIdentifiers({ id: 'u1', username: 'Owner' }, [
    { email: 'owner@example.com', verified: true }, { email: 'unverified@example.com', verified: false }
  ])
  assert.equal(deletionConfirmationMatches('OWNER@example.com', '注销我的账号', identifiers), true)
  assert.equal(deletionConfirmationMatches('Owner', '注销我的账号', identifiers), true)
  for (const [identity, phrase] of [['owner', '注销我的账号'], ['u1', '注销我的账号'], ['unverified@example.com', '注销我的账号'], ['Owner', '注销我的账号 ']]) {
    assert.equal(deletionConfirmationMatches(identity, phrase, identifiers), false)
  }
})
test('accounts without username or verified email use user ID', () => {
  const identifiers = deletionIdentifiers({ id: 'u1' }, [{ email: 'pending@example.com', verified: false }])
  assert.equal(deletionConfirmationMatches('u1', '注销我的账号', identifiers), true)
  assert.equal(deletionConfirmationMatches('', '注销我的账号', []), false)
})
test('deadline always displays Beijing time', () => {
  assert.match(formatDeletionTime('2026-10-15T00:00:00Z'), /2026.*10.*15.*08:00:00/)
})
test('deletion request sends no frontend confirmation fields and cancellation of reauth aborts submission', async () => {
  const previous = api.defaults.adapter
  const calls = []
  api.defaults.adapter = async config => {
    calls.push(config)
    throw new AxiosError('Forbidden', AxiosError.ERR_BAD_REQUEST, config, null, {
      status: 403, config, headers: {}, data: { data: { code: 'REAUTH_REQUIRED', reauth: { methods: ['email'], max_age: 300 } } }
    })
  }
  clearReauth()
  try {
    const request = userAPI.requestAccountDeletion()
    const rejected = assert.rejects(request, error => error.name === 'ReauthCancelledError')
    await new Promise(resolve => setImmediate(resolve))
    assert.equal(reauthState.visible, true)
    cancelReauth()
    await rejected
    assert.equal(calls.length, 1)
    assert.equal(calls[0].data, null)
    assert.equal(calls[0].url, '/user/account-deletion')
  } finally { api.defaults.adapter = previous; clearReauth() }
})
test('verified deletion request replays exactly once without confirmation input', async () => {
  const previous = api.defaults.adapter
  const calls = []
  api.defaults.adapter = async config => {
    calls.push(config)
    if (!config.headers.get('X-Reauth-Token')) throw new AxiosError('Forbidden', AxiosError.ERR_BAD_REQUEST, config, null, {
      status: 403, config, headers: {}, data: { data: { code: 'REAUTH_REQUIRED', reauth: { methods: ['email'] } } }
    })
    return { status: 202, headers: {}, config, data: { data: { status: 'pending_deletion' } } }
  }
  clearReauth()
  try {
    const request = userAPI.requestAccountDeletion()
    await new Promise(resolve => setImmediate(resolve))
    completeReauth({ token: 'verified-token', expires_in: 300 })
    assert.equal((await request).data.status, 'pending_deletion')
    assert.equal(calls.length, 2)
    assert.equal(calls[1].data, null)
  } finally { api.defaults.adapter = previous; clearReauth() }
})

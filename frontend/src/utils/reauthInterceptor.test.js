import assert from 'node:assert/strict'
import test from 'node:test'
import { AxiosError } from 'axios'

import { api, userAPI } from '../api/auth.js'
import { clearReauth, completeReauth, reauthState } from './reauthCoordinator.js'

const forbidden = (config) => new AxiosError('Forbidden', AxiosError.ERR_BAD_REQUEST, config, null, {
  status: 403,
  statusText: 'Forbidden',
  headers: {},
  config,
  data: {
    code: 403,
    message: '需要重新验证',
    data: {
      code: 'REAUTH_REQUIRED',
      reauth: { methods: ['passkey', 'email'], max_age: 300, email_hint: 'o***@example.com' }
    }
  }
})

test('reauth interceptor replays the original request once with the grant', async () => {
  clearReauth()
  const previousAdapter = api.defaults.adapter
  const calls = []
  api.defaults.adapter = async (config) => {
    calls.push(config)
    if (!config.headers?.get?.('X-Reauth-Token')) throw forbidden(config)
    return { data: { code: 200, message: 'success', data: { deleted: true } }, status: 200, statusText: 'OK', headers: {}, config }
  }

  try {
    const pending = userAPI.unbindThirdParty('github')
    await new Promise(resolve => setImmediate(resolve))
    assert.equal(reauthState.visible, true)
    completeReauth({ token: 'grant-token', expires_in: 300 })

    const result = await pending
    assert.equal(result.data.deleted, true)
    assert.equal(calls.length, 2)
    assert.equal(calls[1].headers.get('X-Reauth-Token'), 'grant-token')
  } finally {
    api.defaults.adapter = previousAdapter
    clearReauth()
  }
})

test('reauth interceptor does not loop when the replay is rejected', async () => {
  clearReauth()
  const previousAdapter = api.defaults.adapter
  let calls = 0
  api.defaults.adapter = async (config) => {
    calls += 1
    throw forbidden(config)
  }

  try {
    const pending = userAPI.unbindThirdParty('github')
    await new Promise(resolve => setImmediate(resolve))
    completeReauth({ token: 'grant-token', expires_in: 300 })

    await assert.rejects(pending, error => error.status === 403)
    assert.equal(calls, 2)
    assert.equal(reauthState.visible, false)
  } finally {
    api.defaults.adapter = previousAdapter
    clearReauth()
  }
})

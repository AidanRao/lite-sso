import assert from 'node:assert/strict'
import test from 'node:test'
import { AxiosError } from 'axios'
import { api } from '../api/auth.js'

test('a revoked feature broadcasts its key and preserves the recognizable API error without retrying', async () => {
  const previousWindow = globalThis.window
  const previousAdapter = api.defaults.adapter
  const received = []
  globalThis.window = { dispatchEvent: event => received.push(event) }
  let calls = 0
  api.defaults.adapter = async config => {
    calls++
    throw new AxiosError('Forbidden', AxiosError.ERR_BAD_REQUEST, config, null, {
      status: 403, data: { code: 403, message: '该功能暂未向你开放', data: { code: 'FEATURE_NOT_ENABLED', feature_key: 'profile.audit_logs' } }
    })
  }
  try {
    await assert.rejects(api.get('/user/audit-logs'), error => error.status === 403 && error.data.code === 'FEATURE_NOT_ENABLED')
    assert.equal(calls, 1)
    assert.equal(received.length, 1)
    assert.equal(received[0].type, 'feature-not-enabled')
    assert.equal(received[0].detail, 'profile.audit_logs')
  } finally {
    api.defaults.adapter = previousAdapter
    if (previousWindow === undefined) delete globalThis.window
    else globalThis.window = previousWindow
  }
})

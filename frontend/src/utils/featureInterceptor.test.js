import assert from 'node:assert/strict'
import test from 'node:test'
import { AxiosError } from 'axios'
import { api, setAccessToken, clearAccessToken } from '../api/auth.js'
import { permissions } from './permissions.js'

const response = (config, enabled = true) => ({ data: { data: { is_admin: true, features: { logs: { enabled, stage: 'beta' } } } }, status: 200, statusText: 'OK', headers: {}, config })

test('feature denial updates shared permissions and refreshes without retrying the protected request', async () => {
  const previousAdapter = api.defaults.adapter
  permissions.reset()
  let reads = 0, protectedCalls = 0
  api.defaults.adapter = async config => {
    if (config.url === '/user/permissions') { reads++; return response(config, reads === 1) }
    protectedCalls++
    throw new AxiosError('Forbidden', AxiosError.ERR_BAD_REQUEST, config, null, {
      status: 403, data: { code: 403, message: '该功能暂未向你开放', data: { code: 'FEATURE_NOT_ENABLED', feature_key: 'logs' } }
    })
  }
  try {
    await permissions.load()
    await assert.rejects(api.get('/user/audit-logs'), error => error.status === 403 && error.data.code === 'FEATURE_NOT_ENABLED')
    assert.equal(permissions.state.features.logs.enabled, false)
    await permissions.load()
    assert.equal(reads, 2)
    assert.equal(protectedCalls, 1)
  } finally { api.defaults.adapter = previousAdapter; permissions.reset() }
})

test('login and logout reset cache; ordinary token replacement preserves it', async () => {
  const previousAdapter = api.defaults.adapter
  api.defaults.adapter = async config => response(config)
  try {
    await permissions.load()
    setAccessToken('renewed', { preservePermissions: true })
    assert.equal(permissions.state.loaded, true)
    setAccessToken('new-login')
    assert.equal(permissions.state.loaded, false)
    await permissions.load()
    clearAccessToken()
    assert.equal(permissions.state.loaded, false)
    assert.equal(permissions.state.isAdmin, false)
  } finally { api.defaults.adapter = previousAdapter; clearAccessToken() }
})

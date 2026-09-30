import test from 'node:test'
import assert from 'node:assert/strict'
import { featureEnabled, filterFeatureNavigation } from './features.js'

const navigation = [{ label: 'Account', to: '/profile/account' }, { label: 'Archived', children: [{ label: '操作日志', featureKey: 'profile.audit_logs' }] }]
test('loading or unavailable decisions hide controlled items and empty groups', () => {
  assert.deepEqual(filterFeatureNavigation(navigation, {}), [{ label: 'Account', to: '/profile/account', beta: false }])
  assert.equal(featureEnabled({}, 'profile.audit_logs'), false)
  assert.equal(featureEnabled({}, undefined), true)
})
test('beta is independent of access and navigation source remains unchanged', () => {
  for (const stage of ['beta', 'stable']) {
    const filtered = filterFeatureNavigation(navigation, { 'profile.audit_logs': { enabled: true, stage } })
    assert.equal(filtered[1].children[0].beta, stage === 'beta')
  }
  assert.equal(navigation[1].children[0].beta, undefined)
  const revoked = { 'profile.audit_logs': { enabled: false, stage: 'beta' } }
  assert.equal(filterFeatureNavigation(navigation, revoked).length, 1)
  assert.equal(featureEnabled(revoked, 'profile.audit_logs'), false)
})

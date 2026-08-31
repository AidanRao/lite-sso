import test from 'node:test'
import assert from 'node:assert/strict'
import { auditActionMatches, auditApplicationName, auditDateBounds, auditDetailRows, auditFullTime, auditLoadError, auditRelativeTime, auditSummary, auditSummaryParts, buildAuditQuery, localDateInput } from './auditLog.js'

test('third-party redirects describe distinct business stages rather than HTTP status', () => {
  const event = { outcome: 'success', http_status: 307, details: { provider: 'github', auth_method: 'github' } }
  assert.match(auditSummary({ ...event, action: 'auth.third_party.start' }), /等待授权回调/)
  assert.match(auditSummary({ ...event, action: 'user.third_party.start' }), /尚未完成绑定/)
  assert.match(auditSummary({ ...event, action: 'user.third_party.callback', details: { provider: 'github', completed_steps: ['binding_prepared'] } }), /等待确认/)
  assert.match(auditSummary({ ...event, action: 'user.third_party.bind', details: { provider: 'github', completed_steps: ['binding_created'] } }), /绑定已完成/)
  assert.match(auditSummary({ ...event, action: 'auth.login.third_party', details: { auth_method: 'github', completed_steps: ['session_created'] } }), /通过GitHub完成登录/)
  assert.equal(auditSummary({ ...event, action: 'oauth.authorize', client_id: 'demo-app', details: { completed_steps: ['authorization_code_issued'] } }), '已为应用「demo-app」完成账号授权。')
  assert.equal(auditSummary({ ...event, action: 'auth.login.third_party', outcome: 'failure', reason_code: 'UNCLASSIFIED_REDIRECT' }), '跳转结果未确认')
  assert.doesNotMatch(auditSummary({ ...event, action: 'auth.login.third_party', outcome: 'failure', reason_code: 'PROVIDER_AUTH_FAILED' }), /完成登录/)
})

test('partial completion and denial never become full success', () => {
  assert.equal(auditSummary({ action: 'user.email.add', outcome: 'failure', reason_code: 'MESSAGE_NOT_SENT', details: { completed_steps: ['email_created'] } }), '邮箱已添加，但验证邮件发送失败。')
  assert.match(auditSummary({ action: 'user.password.reset', outcome: 'failure', details: { completed_steps: ['password_updated'] } }), /密码已更新；重置密码未完成/)
  assert.match(auditSummary({ action: 'user.password.update', outcome: 'denied', reason_code: 'REAUTH_REQUIRED' }), /被拒绝：需要重新验证身份/)
  assert.match(auditSummary({ action: 'future.unknown', outcome: 'success' }), /已记录该操作，结果：成功/)
  assert.match(auditSummary({ action: 'user.password.update', outcome: 'failure', reason_code: 'UNKNOWN' }), /未提供可识别的结果说明/)
  assert.match(auditSummary({ action: 'constructor', outcome: 'failure' }), /已记录该操作/)
})

test('application display prefers current metadata and retains IDs for missing applications', () => {
  const event = { action: 'oauth.authorize', outcome: 'success', client_id: 'demo-app', details: { completed_steps: ['authorization_code_issued'] } }
  const withApplication = { ...event, application: { client_id: 'demo-app', name: '示例应用', logo_url: 'https://example.com/logo.png' } }
  assert.equal(auditApplicationName(withApplication), '示例应用')
  assert.equal(auditSummary(withApplication), '已为应用「示例应用」完成账号授权。')
  assert.equal(auditDetailRows(withApplication).find(row => row.label === '应用名称（当前）').value, '示例应用')
  assert.equal(auditApplicationName({ ...event, application: null }), 'demo-app')
  assert.equal(auditSummary({ ...event, application: null }), '已为应用「demo-app」完成账号授权。')
  assert.equal(auditApplicationName({}), '')
  assert.equal(auditSummary({ ...event, client_id: null }), '已完成应用授权。')
})

test('authorization summary embeds an application token with only safe homepage links', () => {
  const event = { action: 'oauth.authorize', outcome: 'success', details: { completed_steps: ['authorization_code_issued'] }, application: { name: '<b>devops-local</b>', logo_url: '/logo.png', homepage_url: 'http://localhost:8080/' } }
  assert.deepEqual(auditSummaryParts(event), [
    { type: 'text', text: '已为应用「' },
    { type: 'application', text: '<b>devops-local</b>', logo: '/logo.png', href: 'http://localhost:8080/' },
    { type: 'text', text: '」完成账号授权。' }
  ])
  for (const homepage of ['', null, 'javascript:alert(1)', 'data:text/html,test', 'file:///tmp/test', 'https://user:password@example.com', '/relative', 'not a URL']) {
    const parts = auditSummaryParts({ ...event, application: { ...event.application, homepage_url: homepage } })
    assert.equal(parts[1].href, '')
  }
  for (const override of [{ outcome: 'failure' }, { reason_code: 'UNCLASSIFIED_REDIRECT' }, { details: {} }]) {
    assert.ok(auditSummaryParts({ ...event, ...override }).every(part => part.type === 'text'))
  }
  const deleted = auditSummaryParts({ ...event, client_id: 'deleted-app', application: null })
  assert.deepEqual(deleted[1], { type: 'application', text: 'deleted-app', logo: '', href: '' })
})

test('frontend translates Chinese search into codes without dropping the original text', () => {
  assert.deepEqual(auditActionMatches('修改密码'), ['user.password.update'])
  assert.ok(auditActionMatches('第三方').includes('auth.login.third_party'))
  assert.deepEqual(auditActionMatches(''), [])
  assert.deepEqual(auditActionMatches('192.0.2.1'), [])
  const query = buildAuditQuery({ q: ' 修改密码 ', outcome: 'failure', period: '30' }, Date.parse('2026-08-31T12:00:00Z'))
  assert.equal(query.q, '修改密码')
  assert.equal(query.actions, 'user.password.update')
  assert.equal(query.outcome, 'failure')
  assert.equal(query.from, '2026-08-01T12:00:00.000Z')
  assert.equal(query.to, '2026-08-31T12:00:00.000Z')
  assert.equal(query.limit, 30)
  assert.throws(() => buildAuditQuery({ q: '字'.repeat(101) }), /100/)
})

test('custom dates use local calendar boundaries and are capped to the rolling window', () => {
  const now = new Date(2026, 7, 31, 12).getTime()
  const bounds = auditDateBounds(now)
  assert.equal(bounds.max, '2026-08-31')
  assert.equal(localDateInput(new Date(2026, 7, 31)), '2026-08-31')
  const query = buildAuditQuery({ period: 'custom', startDate: '2026-08-20', endDate: '2026-08-21' }, now)
  assert.equal(query.from, new Date(2026, 7, 20).toISOString())
  assert.equal(query.to, new Date(2026, 7, 22).toISOString())
  const capped = buildAuditQuery({ period: 'custom', startDate: bounds.min, endDate: bounds.max }, now)
  assert.equal(new Date(capped.from).getTime(), now - 30 * 86400000)
  assert.equal(new Date(capped.to).getTime(), now)
  for (const [startDate, endDate] of [['2026-08-22', '2026-08-21'], ['2026-07-01', '2026-08-02'], ['2026-08-01', '2026-09-01'], ['', ''], ['2026-08-00', '2026-08-10']]) {
    assert.throws(() => buildAuditQuery({ period: 'custom', startDate, endDate }, now))
  }
})

test('date formatting, reason fallback and details stay in the frontend', () => {
  const now = Date.parse('2026-08-31T12:00:00Z')
  assert.equal(auditRelativeTime(now - 2000, now), '刚刚')
  assert.equal(auditRelativeTime(now - 120000, now), '2 分钟前')
  assert.equal(auditRelativeTime(now - 7200000, now), '2 小时前')
  assert.equal(auditRelativeTime(now - 2 * 86400000, now), '2 天前')
  assert.equal(auditRelativeTime('bad'), '—')
  assert.equal(auditFullTime('bad'), '—')
  assert.match(auditFullTime('2026-08-31T12:00:00Z'), /2026/)
  const rows = auditDetailRows({ id: '<script>test</script>', action: 'user.profile.update', reason_code: 'UNKNOWN_CODE', details: { changed_fields: ['username'], completed_steps: ['session_created'], auth_method: 'github' } })
  assert.equal(rows.find(row => row.label === '变更字段').value, '用户名')
  assert.equal(rows.find(row => row.label === '已完成步骤').value, '登录会话已创建')
  assert.equal(rows.find(row => row.label === '原因码').value, 'UNKNOWN_CODE')
  assert.equal(rows.find(row => row.label === '认证方式').value, 'GitHub')
  assert.doesNotMatch(auditLoadError({ status: 500, message: 'private backend details' }), /private/)
})

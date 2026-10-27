import assert from 'node:assert/strict'
import test from 'node:test'
import QRCode from 'qrcode'
import { createQRLoginController } from './qrLogin.js'

const deferred = () => {
  let resolve
  let reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
const flush = async () => { for (let i = 0; i < 10; i++) await Promise.resolve() }
const challenge = code => ({ code, qr_payload: JSON.stringify({ type: 'lite-sso.qr-login', version: 1, issuer: 'https://sso.test', code }), browser_token: `secret-${code}`, expires_at: '2030-01-01T00:05:00Z' })
const setup = overrides => {
  const calls = []
  const timers = new Map()
  let next = 0
  let clock = Date.parse('2030-01-01T00:00:00Z')
  const changes = []
  const api = {
    getQRCode: async () => ({ data: challenge('first') }),
    cancelQRCode: async (...args) => calls.push(['cancel', ...args]),
    checkQRCode: async (...args) => { calls.push(['poll', ...args]); return { data: { status: 'pending' } } },
    completeQRCode: async (...args) => { calls.push(['complete', ...args]); return { data: 'logged-in' } },
    ...overrides
  }
  const controller = createQRLoginController({ api, render: async payload => QRCode.toDataURL(payload),
    onChange: state => changes.push(state), onComplete: result => calls.push(['done', result]),
    now: () => clock, schedule: (fn, delay) => { const id = ++next; timers.set(id, { fn, delay }); return id },
    cancelTimer: id => timers.delete(id)
  })
  return { controller, calls, changes, timers, advance: milliseconds => { clock += milliseconds } }
}

test('renders a QR image and passes only browser credentials to polling and completion', async () => {
  const fixture = setup({ checkQRCode: async () => ({ data: { status: 'confirmed', login_ticket: 'ticket' } }) })
  await fixture.controller.start('/oauth/authorize?state=original')
  await flush()
  assert.match(fixture.changes.find(state => state.image)?.image || '', /^data:image\/png;base64,/)
  assert.deepEqual(fixture.calls[0], ['complete', { code: 'first', login_ticket: 'ticket' }, 'secret-first'])
  assert.equal(fixture.calls[1][0], 'done')
  assert.equal(fixture.timers.size, 0)
})

test('refresh cancels the old request and ignores its late confirmation', async () => {
  const oldPoll = deferred()
  let generated = 0
  const fixture = setup({ getQRCode: async () => ({ data: challenge(++generated === 1 ? 'old' : 'new') }),
    checkQRCode: code => code === 'old' ? oldPoll.promise : Promise.resolve({ data: { status: 'pending' } }) })
  await fixture.controller.start('/profile')
  await fixture.controller.start('/profile')
  oldPoll.resolve({ data: { status: 'confirmed', login_ticket: 'stale' } })
  await flush()
  assert.deepEqual(fixture.calls[0], ['cancel', 'old', 'secret-old'])
  assert.equal(fixture.calls.some(call => call[0] === 'complete'), false)
  fixture.controller.stop()
  assert.equal(fixture.timers.size, 0)
  assert.deepEqual(fixture.calls.at(-1), ['cancel', 'new', 'secret-new'])
})

test('leaving during generation cancels the late request without rendering it', async () => {
  const generation = deferred()
  const fixture = setup({ getQRCode: () => generation.promise })
  const pending = fixture.controller.start('/profile')
  fixture.controller.stop()
  generation.resolve({ data: challenge('late') })
  await pending
  assert.deepEqual(fixture.calls, [['cancel', 'late', 'secret-late']])
  assert.equal(fixture.changes.at(-1).image, '')
})

test('polling is serialized and expiry uses the server timestamp', async () => {
  const firstPoll = deferred()
  let polls = 0
  const fixture = setup({ checkQRCode: async () => { polls++; return firstPoll.promise } })
  await fixture.controller.start('/profile')
  assert.equal(polls, 1)
  assert.equal([...fixture.timers.values()].some(timer => timer.delay === 2000), false)
  firstPoll.resolve({ data: { status: 'scanned' } })
  await flush()
  assert.equal(fixture.changes.at(-1).message, '已扫描，请在 App 中确认')
  fixture.advance(300_000)
  const countdown = [...fixture.timers.values()].find(timer => timer.delay === 1000)
  countdown.fn()
  assert.equal(fixture.changes.at(-1).seconds, 0)
  assert.equal(fixture.changes.at(-1).image, '')
  assert.equal(fixture.timers.size, 0)
})

test('cancelled requests and completion failures do not repeat session creation', async () => {
  const cancelled = setup({ checkQRCode: async () => ({ data: { status: 'cancelled' } }) })
  await cancelled.controller.start('/profile')
  await flush()
  assert.equal(cancelled.changes.at(-1).message, '登录已取消，请刷新二维码')
  assert.equal(cancelled.timers.size, 0)
  const failed = setup({ checkQRCode: async () => ({ data: { status: 'confirmed', login_ticket: 'ticket' } }),
    completeQRCode: async () => { throw new Error('登录失败，请重新生成二维码') } })
  await failed.controller.start('/profile')
  await flush()
  assert.equal(failed.changes.at(-1).message, '登录失败，请重新生成二维码')
  assert.equal(failed.timers.size, 0)
})

test('a late confirmation after local expiry cannot resurrect the request', async () => {
  const late = deferred()
  const fixture = setup({ checkQRCode: () => late.promise })
  await fixture.controller.start('/profile')
  fixture.advance(300_000)
  const timer = [...fixture.timers.values()].find(timer => timer.delay === 1000)
  timer.fn()
  late.resolve({ data: { status: 'confirmed', login_ticket: 'late' } })
  await flush()
  assert.equal(fixture.calls.some(call => call[0] === 'complete'), false)
  assert.equal(fixture.changes.at(-1).image, '')
})

test('QR login defaults to remembered sessions and preserves an explicit backend choice', async () => {
  const generated = []
  const fixture = setup({ getQRCode: async (...args) => { generated.push(args); return { data: challenge('choice') } } })
  await fixture.controller.start('/profile')
  await fixture.controller.start('/profile', false)
  assert.deepEqual(generated, [['/profile', true], ['/profile', false]])
  fixture.controller.stop()
})

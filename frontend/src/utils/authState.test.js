import assert from 'node:assert/strict'
import test from 'node:test'
import { effectScope, reactive, ref } from 'vue'
import { useAuthSubmission, useCodeDelivery, useDeadline, useEmailChallenge } from './authState.js'

const deferred = () => {
  let resolve
  let reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
const flush = async () => { for (let i = 0; i < 10; i++) await Promise.resolve() }
const clock = () => {
  let now = 10000
  let sequence = 0
  const timers = new Map()
  return {
    options: { now: () => now, schedule: fn => { timers.set(++sequence, fn); return sequence }, cancel: id => timers.delete(id) },
    timers,
    advance: milliseconds => {
      now += milliseconds
      const pending = [...timers.values()]
      timers.clear()
      pending.forEach(fn => fn())
    }
  }
}

test('deadline retains the original unlock time across a delayed tick and cleans up on disposal', () => {
  const time = clock()
  const scope = effectScope()
  const deadline = scope.run(() => useDeadline(time.options))
  deadline.start(60)
  assert.equal(deadline.seconds.value, 60)
  time.advance(58000)
  assert.equal(deadline.seconds.value, 2)
  time.advance(2000)
  assert.equal(deadline.seconds.value, 0)
  assert.equal(time.timers.size, 0)
  deadline.start(30)
  scope.stop()
  assert.equal(time.timers.size, 0)
})

test('email challenge uses server cooldown and expiry, survives unrelated mode changes and accepts a resend', () => {
  const time = clock()
  const scope = effectScope()
  const email = ref('person@example.com')
  const challenge = scope.run(() => useEmailChallenge(email, time.options))
  challenge.visible.value = true
  challenge.accept({ challenge_id: 'first', resend_after: 15, expires_in: 90 })
  assert.equal(challenge.countdown.value, 15)
  challenge.code.value = '123456'
  time.advance(16000)
  assert.equal(challenge.countdown.value, 0)
  assert.equal(challenge.challengeId.value, 'first')
  assert.equal(challenge.valid.value, true)
  challenge.visible.value = true
  challenge.accept({ challenge_id: 'second', resend_after: 10, expires_in: 120 })
  assert.equal(challenge.challengeId.value, 'second')
  assert.equal(challenge.code.value, '')
  time.advance(120000)
  assert.equal(challenge.valid.value, false)
  assert.equal(challenge.challengeId.value, '')
  scope.stop()
  assert.equal(time.timers.size, 0)
})

test('changing email invalidates code, challenge, dialog and timers synchronously', () => {
  const time = clock()
  const scope = effectScope()
  const email = ref('person@example.com')
  const challenge = scope.run(() => useEmailChallenge(email, time.options))
  challenge.visible.value = true
  challenge.accept({ challenge_id: 'first', resend_after: 30, expires_in: 300 })
  challenge.code.value = '123456'
  email.value = ' PERSON@example.com '
  assert.equal(challenge.challengeId.value, 'first')
  email.value = 'other@example.com'
  assert.equal(challenge.challengeId.value, '')
  assert.equal(challenge.code.value, '')
  assert.equal(challenge.visible.value, false)
  assert.equal(challenge.countdown.value, 0)
  assert.equal(time.timers.size, 0)
  challenge.accept({ challenge_id: 'late', resend_after: 60, expires_in: 300 })
  assert.equal(challenge.valid.value, false)
  scope.stop()
})

test('submission gate blocks duplicate validation and keeps successful navigation locked', async () => {
  const scope = effectScope()
  const gate = scope.run(() => useAuthSubmission())
  const result = deferred()
  let calls = 0
  const first = gate.run(async () => { calls++; return result.promise })
  await gate.run(async () => { calls++ })
  assert.equal(calls, 1)
  result.resolve(false)
  await first
  assert.equal(gate.busy.value, false)
  await gate.run(async () => true)
  await gate.run(async () => { calls++ })
  assert.equal(calls, 1)
  assert.equal(gate.busy.value, true)
  scope.stop()
})

test('submission completion is stale after page disposal', async () => {
  const scope = effectScope()
  const gate = scope.run(() => useAuthSubmission())
  const response = deferred()
  let navigated = false
  const pending = gate.run(async isCurrent => { await response.promise; navigated = isCurrent() })
  scope.stop()
  response.resolve()
  await pending
  assert.equal(navigated, false)
})

const deliveryFixture = overrides => {
  const scope = effectScope()
  const context = reactive({ visible: true, email: 'person@example.com', purpose: 'LOGIN' })
  const success = []
  const sent = []
  const api = {
    getCaptcha: async () => ({ data: { captcha_id: 'captcha', captcha_png_base64: 'data:image/png;base64,test' } }),
    sendEmailCode: async data => { sent.push(data); return { data: { challenge_id: 'challenge' } } },
    ...overrides
  }
  const state = scope.run(() => useCodeDelivery({ context: () => context, api,
    onSuccess: data => success.push(data), onClose: () => { context.visible = false } }))
  return { scope, context, success, sent, state }
}

test('code delivery sends one request with its original purpose, including password reset', async () => {
  const response = deferred()
  let calls = 0
  let request
  const f = deliveryFixture({ sendEmailCode: data => { calls++; request = data; return response.promise } })
  f.context.purpose = 'PASSWORD_RESET'
  await flush()
  f.state.captcha.value = '1234'
  const pending = f.state.submit()
  await f.state.submit()
  assert.equal(calls, 1)
  assert.deepEqual(request, { email: 'person@example.com', purpose: 'PASSWORD_RESET', captcha_id: 'captcha', captcha: '1234' })
  response.resolve({ data: { challenge_id: 'reset' } })
  await pending
  assert.deepEqual(f.success, [{ challenge_id: 'reset' }])
  assert.equal(f.context.visible, false)
  f.scope.stop()
})

test('close, email replacement, purpose replacement and unmount all discard pending sends', async () => {
  for (const invalidate of [f => f.state.close(), f => { f.context.email = 'new@example.com' },
    f => { f.context.purpose = 'REGISTER' }, f => f.scope.stop()]) {
    const response = deferred()
    const f = deliveryFixture({ sendEmailCode: () => response.promise })
    await flush()
    f.state.captcha.value = '1234'
    const pending = f.state.submit()
    invalidate(f)
    response.resolve({ data: { challenge_id: 'stale' } })
    await pending
    assert.deepEqual(f.success, [])
    assert.equal(f.state.loading.value, false)
    f.scope.stop()
  }
})

test('an old captcha cannot overwrite a reopened dialog; send errors stay visible while refreshing', async () => {
  const old = deferred()
  let count = 0
  const f = deliveryFixture({ getCaptcha: () => ++count === 1 ? old.promise : Promise.resolve({ data: { captcha_id: 'new', captcha_png_base64: 'new-image' } }),
    sendEmailCode: async () => { throw new Error('发送过于频繁') } })
  f.state.close()
  f.context.visible = true
  await flush()
  old.resolve({ data: { captcha_id: 'old', captcha_png_base64: 'old-image' } })
  await flush()
  assert.equal(f.state.image.value, 'new-image')
  f.state.captcha.value = '1234'
  await f.state.submit()
  assert.equal(f.state.error.value, '发送过于频繁')
  assert.equal(f.state.loading.value, false)
  assert.equal(f.state.captcha.value, '')
  f.scope.stop()
})

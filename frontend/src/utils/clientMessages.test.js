import test from 'node:test'
import assert from 'node:assert/strict'
import { createMessageRun, parseMessageSettings, sendMessageRun, setMessageSelection } from './clientMessages.js'

const settings = () => parseMessageSettings(' sender ', ' notice ', '{"count":2,"enabled":true,"items":[null,{"name":"通知"}]}')
const users = count => Array.from({ length: count }, (_, index) => ({ user_id: `user-${index}`, email: `${index}@example.com` }))
const createRun = count => createMessageRun(users(count), settings(), (() => { let n = 0; return () => `key-${n++}` })())

test('message settings preserve JSON types and enforce an object and size limits', () => {
  assert.equal(settings().sender_key, 'sender')
  assert.equal(settings().template_key, 'notice')
  assert.deepEqual(settings().variables, { count: 2, enabled: true, items: [null, { name: '通知' }] })
  for (const text of ['null', '[]', '"text"', '2', 'true', '{']) assert.throws(() => parseMessageSettings('s', 't', text))
  assert.throws(() => parseMessageSettings(' ', 't', '{}'))
  assert.throws(() => parseMessageSettings('s', 'x'.repeat(101), '{}'))
  assert.throws(() => parseMessageSettings('s', 't', JSON.stringify({ body: '中'.repeat(23000) })))
})

test('selection persists across pages, excludes missing email and deselects only the requested page', () => {
  const selection = new Map()
  const pageOne = [...users(2), { user_id: 'no-email', email: null }]
  const pageTwo = [{ user_id: 'other', email: 'other@example.com' }]
  setMessageSelection(selection, pageOne, true)
  setMessageSelection(selection, pageTwo, true)
  assert.deepEqual([...selection.keys()], ['user-0', 'user-1', 'other'])
  setMessageSelection(selection, pageOne, false)
  assert.deepEqual([...selection.keys()], ['other'])
})

test('a run freezes recipients and shared parameters independently of subsequent edits', () => {
  const inputUsers = users(2)
  const inputSettings = settings()
  const run = createMessageRun(inputUsers, inputSettings, () => 'key')
  inputUsers[0].email = 'changed@example.com'
  inputSettings.variables.items[1].name = 'changed'
  assert.equal(run.entries[0].email, '0@example.com')
  assert.equal(run.settings.variables.items[1].name, '通知')
})

test('queue caps concurrency at three and isolates failures and unknown outcomes', async () => {
  const run = createRun(8)
  let active = 0
  let maximum = 0
  await sendMessageRun(run, async entry => {
    active++
    maximum = Math.max(maximum, active)
    await new Promise(resolve => setTimeout(resolve, 5))
    active--
    if (entry.user_id === 'user-1') throw Object.assign(new Error('rejected'), { status: 502, data: { code: 'MESSAGE_SUBMISSION_REJECTED' } })
    if (entry.user_id === 'user-2') throw new Error('network disconnected')
    if (entry.user_id === 'user-3') throw Object.assign(new Error('timeout'), { status: 504 })
  })
  assert.equal(maximum, 3)
  assert.deepEqual(run.entries.map(entry => entry.status), ['accepted', 'failed', 'unknown', 'unknown', 'accepted', 'accepted', 'accepted', 'accepted'])
})

test('retry skips accepted recipients and preserves keys and parameters', async () => {
  const run = createRun(3)
  await sendMessageRun(run, async entry => { if (entry.user_id !== 'user-0') throw new Error('timeout') })
  const attempts = []
  await sendMessageRun(run, async (entry, shared) => attempts.push({ key: entry.key, shared }))
  assert.deepEqual(attempts.map(attempt => attempt.key), ['key-1', 'key-2'])
  assert.ok(attempts.every(attempt => attempt.shared === run.settings))
  assert.ok(run.entries.every(entry => entry.status === 'accepted'))
  assert.notEqual(createMessageRun(users(1), settings()).entries[0].key, createMessageRun(users(1), settings()).entries[0].key)
})

test('leaving stops further scheduling while in-flight submissions finish', async () => {
  const run = createRun(10)
  let stopped = false
  const calls = []
  await sendMessageRun(run, async entry => {
    calls.push(entry.user_id)
    stopped = true
  }, { shouldStop: () => stopped })
  assert.equal(calls.length, 1)
  assert.equal(run.entries[0].status, 'accepted')
  assert.equal(run.entries[1].status, 'pending')
})

test('permission loss stops remaining recipients', async () => {
  const run = createRun(10)
  let calls = 0
  let denied = 0
  await sendMessageRun(run, async () => {
    calls++
    throw Object.assign(new Error('permission lost'), { status: 403 })
  }, { onAccessError: () => { denied++ } })
  assert.equal(calls, 3)
  assert.equal(denied, 1)
  assert.ok(run.entries.slice(3).every(entry => entry.status === 'pending'))
})

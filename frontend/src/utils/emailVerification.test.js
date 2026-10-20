import test from 'node:test'
import assert from 'node:assert/strict'

import { forgetEmailVerificationToken, getEmailVerificationToken, readEmailVerificationToken, rememberEmailVerificationToken } from './emailVerification.js'

test('email verification token is read only from the fragment parameters', () => {
  assert.equal(getEmailVerificationToken('#token=abc_DEF-123'), 'abc_DEF-123')
  assert.equal(getEmailVerificationToken('#other=value&token=second'), 'second')
  assert.equal(getEmailVerificationToken(''), '')
  assert.equal(getEmailVerificationToken('#token='), '')
})

test('verification fragment survives login locally without being included in a redirect query', () => {
  const values = new Map()
  const storage = {
    setItem: (key, value) => values.set(key, value),
    getItem: key => values.get(key) ?? null,
    removeItem: key => values.delete(key)
  }
  rememberEmailVerificationToken(getEmailVerificationToken('#token=one-time-secret'), storage)
  assert.equal(readEmailVerificationToken(storage), 'one-time-secret')
  forgetEmailVerificationToken(storage)
  assert.equal(readEmailVerificationToken(storage), '')
})

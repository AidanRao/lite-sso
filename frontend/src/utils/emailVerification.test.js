import test from 'node:test'
import assert from 'node:assert/strict'

import { getEmailVerificationToken } from './emailVerification.js'

test('email verification token is read only from the fragment parameters', () => {
  assert.equal(getEmailVerificationToken('#token=abc_DEF-123'), 'abc_DEF-123')
  assert.equal(getEmailVerificationToken('#other=value&token=second'), 'second')
  assert.equal(getEmailVerificationToken(''), '')
  assert.equal(getEmailVerificationToken('#token='), '')
})

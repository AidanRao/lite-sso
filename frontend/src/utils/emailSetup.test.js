import assert from 'node:assert/strict'
import test from 'node:test'
import { getEmailSetupRedirect } from './emailSetup.js'

test('email setup continuation preserves OAuth parameters and rejects browser-normalized external targets', () => {
  const original = '/oauth/authorize?client_id=demo&redirect_uri=https%3A%2F%2Fapp.test%2Fcb&state=original%2Bstate&code_challenge=a&code_challenge_method=S256'
  const query = new URLSearchParams({ redirect: original })
  assert.equal(getEmailSetupRedirect(query.get('redirect')), original)
  for (const target of [undefined, [], '', 'https://evil.test', '//evil.test', '/\\evil.test', '/\n/evil.test', '/\t/evil.test', '/setup-email?redirect=%2Fprofile', '/setup-email/', '/other/../setup-email']) {
    assert.equal(getEmailSetupRedirect(target), '/profile', String(target))
  }
})

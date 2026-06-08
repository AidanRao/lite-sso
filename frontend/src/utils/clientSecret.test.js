import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { generateClientSecret, maskClientSecret } from './clientSecret.js'

describe('generateClientSecret', () => {
  it('generates a url-safe random secret with the expected length', () => {
    const secret = generateClientSecret()

    assert.equal(secret.length, 43)
    assert.match(secret, /^[A-Za-z0-9_-]+$/)
  })

  it('generates a different secret for repeated calls', () => {
    const secrets = new Set(Array.from({ length: 8 }, () => generateClientSecret()))

    assert.equal(secrets.size, 8)
  })
})

describe('maskClientSecret', () => {
  it('masks an existing secret with the same length', () => {
    assert.equal(maskClientSecret('abc123'), '******')
  })

  it('keeps an empty secret empty', () => {
    assert.equal(maskClientSecret(''), '')
  })
})

import assert from 'node:assert/strict'
import test from 'node:test'

import { submitGlobalLogout } from './logout.js'

test('submitGlobalLogout posts logout form so the browser renders the response page', () => {
  const originalDocument = globalThis.document
  const appended = []
  let submitted = false

  globalThis.document = {
    body: {
      appendChild: (node) => {
        appended.push(node)
      }
    },
    createElement: (tagName) => {
      assert.equal(tagName, 'form')
      return {
        style: {},
        submit: () => {
          submitted = true
        }
      }
    }
  }

  try {
    submitGlobalLogout('/login')

    assert.equal(appended.length, 1)
    assert.equal(appended[0].method, 'POST')
    assert.equal(appended[0].action, '/api/auth/logout?redirect=%2Flogin')
    assert.equal(appended[0].style.display, 'none')
    assert.equal(submitted, true)
  } finally {
    globalThis.document = originalDocument
  }
})

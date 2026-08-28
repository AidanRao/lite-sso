import { startAuthentication } from '@simplewebauthn/browser'
import { passkeyAPI } from '../api/auth'

let cachedGrant = null
let authenticationPromise = null

export class PasskeyCancelledError extends Error {
  constructor() {
    super('Passkey 验证已取消')
    this.name = 'PasskeyCancelledError'
  }
}

export const isPasskeyCancelled = (error) => error instanceof PasskeyCancelledError

export const clearPasskeyReauth = () => {
  cachedGrant = null
}

export const ensurePasskeyReauth = async () => {
  if (cachedGrant && Date.now() < cachedGrant.expiresAt) {
    return cachedGrant.token
  }
  if (!authenticationPromise) {
    authenticationPromise = authenticate().finally(() => {
      authenticationPromise = null
    })
  }
  return authenticationPromise
}

export const withPasskeyReauth = async (operation) => {
  let token = cachedGrant && Date.now() < cachedGrant.expiresAt ? cachedGrant.token : ''
  if (!token) {
    token = await ensurePasskeyReauth()
  }
  try {
    return await operation(token)
  } catch (error) {
    const code = error?.data?.code
    if (code !== 'REAUTH_REQUIRED' && code !== 'REAUTH_TOKEN_INVALID') {
      throw error
    }
    clearPasskeyReauth()
    const refreshedToken = await ensurePasskeyReauth()
    return operation(refreshedToken)
  }
}

const authenticate = async () => {
  if (!window.PublicKeyCredential) {
    throw new Error('当前浏览器不支持 Passkey')
  }
  const optionsResult = await passkeyAPI.reauthOptions()
  const payload = optionsResult?.data || optionsResult
  try {
    const response = await startAuthentication({ optionsJSON: payload.options?.publicKey || payload.options })
    const verifyResult = await passkeyAPI.reauthVerify({ ceremony_id: payload.ceremony_id, response })
    const grant = verifyResult?.data || verifyResult
    cachedGrant = {
      token: grant.token,
      expiresAt: Date.now() + Math.max(0, Number(grant.expires_in || 0) - 2) * 1000
    }
    return cachedGrant.token
  } catch (error) {
    if (error?.name === 'NotAllowedError' || error?.name === 'AbortError') {
      throw new PasskeyCancelledError()
    }
    throw error
  }
}

const defaultSecretBytes = 32

export const generateClientSecret = (byteLength = defaultSecretBytes) => {
  const bytes = new Uint8Array(byteLength)
  globalThis.crypto.getRandomValues(bytes)

  let binary = ''
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte)
  })

  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

export const maskClientSecret = (secret) => {
  return secret ? '*'.repeat(secret.length) : ''
}

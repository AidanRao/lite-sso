export const passwordPolicyMessage = '密码需为10至256位，且同时包含英文字符和数字'
export const passwordLengthMessage = '需为10至256位'
export const passwordLetterMessage = '需包含英文字符'
export const passwordDigitMessage = '需包含数字'

export const passwordPolicyError = (password) => {
  const length = [...password].length
  if (length < 10 || length > 256) return passwordLengthMessage
  if (!/[A-Za-z]/.test(password)) return passwordLetterMessage
  if (!/[0-9]/.test(password)) return passwordDigitMessage
  return ''
}

export const meetsPasswordPolicy = (password) => passwordPolicyError(password) === ''

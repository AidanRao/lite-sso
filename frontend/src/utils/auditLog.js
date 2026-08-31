// All audit presentation and search vocabulary lives here, not in the API.
export const auditCopy = {
  title: '操作日志', description: '查看最近 30 天内与当前账号关联的操作记录。最新记录可能延迟显示。',
  recent: '最近事件', search: '搜索操作、平台、IP 或目标 ID', filters: '筛选', refresh: '刷新',
  loading: '正在加载操作日志…', empty: '最近 30 天暂无可显示的操作记录。', noMatches: '没有符合当前条件的操作记录。',
  retry: '重新加载', loadMore: '加载更多', loadingMore: '正在加载…', end: '已显示全部匹配记录',
  loadFailed: '操作日志加载失败，请稍后重试。', invalidQuery: '查询条件无效，请检查筛选条件后重试。',
  invalidDates: '请选择最近 30 天内有效的起止日期。', longSearch: '搜索内容不能超过 100 个字符。',
  searchSubmit: '搜索', clear: '清除筛选', result: '操作结果', period: '时间范围', from: '开始日期', to: '结束日期',
  details: '查看详情', hideDetails: '收起详情', unknownDevice: '未知设备', unknownIP: '未知 IP', user: '你', placeholder: '—',
  applicationCurrent: '应用名称和 Logo 为当前信息'
}

export const outcomeOptions = [
  { value: '', label: '全部结果' }, { value: 'success', label: '成功' },
  { value: 'failure', label: '失败' }, { value: 'denied', label: '拒绝' }
]
export const periodOptions = [
  { value: '30', label: '最近 30 天' }, { value: '7', label: '最近 7 天' }, { value: 'custom', label: '自定义日期' }
]

export const auditActions = {
  'auth.login.password': '密码登录', 'auth.login.email': '邮箱验证码登录', 'auth.login.qr': '扫码登录',
  'auth.email.send': '发送登录验证码', 'auth.qr.scan': '扫描登录二维码', 'auth.qr.confirm': '确认扫码登录', 'auth.logout': '退出登录',
  'auth.third_party.start': '发起第三方登录', 'auth.login.third_party': '第三方登录', 'oauth.authorize': '应用授权',
  'user.register': '注册账号', 'user.password.reset': '重置密码', 'user.profile.update': '修改账号资料',
  'user.password.update': '修改密码', 'user.avatar.update': '修改头像', 'user.email.add': '添加邮箱',
  'user.email.verify': '验证邮箱', 'user.email.verification.send': '发送邮箱验证邮件', 'user.email.primary': '切换主邮箱',
  'user.email.delete': '删除邮箱', 'user.device.revoke': '撤销登录设备', 'user.third_party.start': '发起第三方绑定',
  'user.third_party.callback': '准备第三方绑定预览', 'user.third_party.bind': '确认第三方绑定', 'user.third_party.unbind': '解除第三方绑定',
  'user.passkey.email.send': '发送 Passkey 注册验证码', 'user.passkey.registration.start': '发起 Passkey 注册',
  'user.passkey.create': '注册 Passkey', 'user.passkey.rename': '修改 Passkey 名称', 'user.passkey.delete': '删除 Passkey',
  'user.reauth.passkey.start': '发起 Passkey 重新验证', 'user.reauth.passkey.verify': 'Passkey 重新验证',
  'user.reauth.email.send': '发送重新验证验证码', 'user.reauth.email.verify': '邮箱重新验证',
  'admin.client.create': '创建应用', 'admin.client.update': '修改应用', 'admin.client.logo.update': '修改应用 Logo',
  'admin.client.logo.clear': '删除应用 Logo', 'admin.client.secret.view': '查看应用密钥'
}

const reasons = {
  OK: '操作成功', UNCLASSIFIED_REDIRECT: '跳转结果未确认', OPERATION_FAILED: '操作未完成', REQUEST_ABORTED: '请求中断', PANIC: '服务处理异常',
  UNAUTHORIZED: '尚未登录或会话已失效', AUTHENTICATION_REQUIRED: '需要登录', ADMIN_REQUIRED: '需要管理员权限',
  AUTHORIZATION_INCOMPLETE: '第三方授权已取消或未完成', DEVICE_MISMATCH: '登录设备发生变化',
  USER_NOT_FOUND: '账号不存在', USER_INACTIVE: '账号已停用', EMAIL_EXISTS: '邮箱已被使用', USERNAME_EXISTS: '用户名已被使用',
  AVATAR_STORAGE_UNAVAILABLE: '头像存储暂不可用', EMAIL_ALREADY_ADDED: '邮箱已添加', EMAIL_INVALID: '邮箱格式无效',
  EMAIL_LIMIT_REACHED: '邮箱数量已达上限', EMAIL_NOT_FOUND: '邮箱不存在', EMAIL_NOT_VERIFIED: '邮箱尚未验证',
  PRIMARY_EMAIL_DELETE: '不能删除主邮箱', EMAIL_VERIFICATION_INVALID: '邮箱验证无效', EMAIL_VERIFICATION_EXPIRED: '邮箱验证已过期',
  EMAIL_VERIFICATION_RESEND: '请重新发送邮箱验证邮件', LAST_LOGIN_METHOD: '不能移除最后一种登录方式',
  INVALID_CREDENTIALS: '登录凭据无效', CURRENT_PASSWORD_INVALID: '当前密码错误', PASSWORD_NOT_SET: '尚未设置密码',
  PASSWORD_LENGTH_INVALID: '密码长度不符合要求', PASSWORD_LETTER_REQUIRED: '密码需要包含字母', PASSWORD_DIGIT_REQUIRED: '密码需要包含数字',
  INVALID_OTP: '验证码无效', OTP_EXPIRED: '验证码已过期', OTP_ATTEMPTS_EXCEEDED: '验证码尝试次数过多', CHALLENGE_INVALID: '验证请求已失效',
  INVALID_CAPTCHA: '图形验证码无效', CAPTCHA_REQUIRED: '需要完成图形验证', RATE_LIMITED: '请求过于频繁',
  SESSION_REVOKED: '会话已撤销', DEVICE_NOT_FOUND: '登录设备不存在', CURRENT_DEVICE: '不能通过此操作撤销当前设备',
  REFRESH_TOKEN_INVALID: '登录状态已失效', ACCOUNT_LOCKED: '账号暂时锁定', MESSAGE_NOT_SENT: '验证消息发送失败', INVALID_REDIRECT: '跳转地址无效',
  PASSKEY_REQUIRED: '需要使用 Passkey', EMAIL_REQUIRED_FOR_PASSKEY: '需要先设置邮箱', WEBAUTHN_CEREMONY_INVALID: 'Passkey 验证请求无效',
  REAUTH_REQUIRED: '需要重新验证身份', REAUTH_TOKEN_INVALID: '重新验证状态已失效', REAUTH_METHOD_UNAVAILABLE: '重新验证方式不可用',
  PASSKEY_NOT_FOUND: 'Passkey 不存在', PASSKEY_NAME_INVALID: 'Passkey 名称无效', PASSKEY_CLONE_WARNING: 'Passkey 安全验证未通过',
  INVALID_PROVIDER: '不支持的第三方平台', PROVIDER_AUTH_FAILED: '第三方身份验证失败', THIRD_PARTY_ALREADY_BOUND: '已绑定此第三方登录方式',
  THIRD_PARTY_BOUND_TO_ANOTHER: '第三方账号已绑定其他账号', THIRD_PARTY_NOT_BOUND: '尚未绑定该平台', THIRD_PARTY_BINDING_NOT_FOUND: '绑定预览不存在或已过期',
  OAUTH_CLIENT_EXISTS: '应用已存在', OAUTH_CLIENT_NOT_FOUND: '应用不存在', INVALID_OAUTH_CLIENT: '应用信息无效', LOGO_STORAGE_UNAVAILABLE: '应用 Logo 存储暂不可用',
  QR_CODE_EXPIRED: '登录二维码已过期', QR_CODE_INVALID_STATUS: '登录二维码状态无效', QR_CODE_INVALID_USER: '扫码账号不匹配', QR_CODE_INVALID_TICKET: '扫码登录凭据无效'
}
const authMethods = { password: '密码', email_otp: '邮箱验证码', qr_code: '扫码', github: 'GitHub', feishu: '飞书', passkey: 'Passkey' }
const providers = { github: 'GitHub', feishu: '飞书' }
const targets = { user: '账号', email: '邮箱', device: '登录设备', provider: '第三方平台', passkey: 'Passkey', oauth_client: '应用' }
const fields = { username: '用户名', avatar: '头像', password: '密码', email: '邮箱', verified: '邮箱验证状态', is_primary: '主邮箱', name: '名称', provider: '第三方绑定', passkey: 'Passkey', session: '会话', logo: 'Logo', client_secret: '应用密钥', redirect_uri: '回调地址', logout_uri: '退出地址', homepage_url: '主页地址', is_active: '启用状态', description: '描述' }
const steps = { user_created: '账号已创建', session_created: '登录会话已创建', password_updated: '密码已更新', sessions_revoked: '会话已撤销', email_created: '邮箱已添加', verification_sent: '验证邮件已发送', email_verified: '邮箱已验证', binding_prepared: '绑定预览已准备', binding_created: '第三方绑定已完成', authorization_code_issued: '应用授权码已签发' }
const dayMS = 24 * 60 * 60 * 1000
const known = (map, key) => Object.hasOwn(map, key) ? map[key] : ''
const valueOrDash = (value) => value === null || value === undefined || value === '' ? auditCopy.placeholder : String(value)

export function auditOutcomeLabel(outcome) {
  return outcomeOptions.find(item => item.value === outcome && outcome)?.label || '结果未知'
}

export function auditApplicationName(event) {
  return event.application?.name || event.client_id || ''
}

export function auditReasonLabel(code) {
  if (known(reasons, code)) return reasons[code]
  if (/^HTTP_[1-5]\d{2}$/.test(code || '')) return `请求未完成（HTTP ${code.slice(5)}）`
  return '未提供可识别的结果说明'
}

export function auditActionMatches(search) {
  const words = String(search || '').trim().toLocaleLowerCase().split(/\s+/).filter(Boolean)
  if (!words.length) return []
  return Object.entries(auditActions).filter(([code, label]) => words.every(word => `${code} ${label}`.toLocaleLowerCase().includes(word))).map(([code]) => code)
}

export function auditSummary(event) {
  return auditSummaryParts(event).map(part => part.text).join('')
}

export function auditSummaryParts(event) {
  const content = auditSummaryContent(event)
  return typeof content === 'string' ? [{ type: 'text', text: content }] : content
}

function applicationHomepage(value) {
  try {
    const url = new URL(value)
    return ['https:', 'http:'].includes(url.protocol) && !url.username && !url.password ? url.href : ''
  } catch {
    return ''
  }
}

function auditSummaryContent(event) {
  const title = known(auditActions, event.action)
  const details = event.details || {}
  const completed = Array.isArray(details.completed_steps) ? details.completed_steps : []
  const provider = known(providers, details.provider)
  if (event.reason_code === 'UNCLASSIFIED_REDIRECT') return reasons.UNCLASSIFIED_REDIRECT
  if (!title) return `已记录该操作，结果：${auditOutcomeLabel(event.outcome)}。`
  if (event.outcome !== 'success') {
    if (event.action === 'user.email.add' && completed.includes('email_created') && !completed.includes('verification_sent')) return '邮箱已添加，但验证邮件发送失败。'
    const facts = completed.map(step => known(steps, step)).filter(Boolean)
    return `${facts.length ? `${facts.join('，')}；` : ''}${title}${event.outcome === 'denied' ? '被拒绝' : '未完成'}：${auditReasonLabel(event.reason_code)}。`
  }
  if (event.action === 'auth.third_party.start') return `已发起${provider ? ` ${provider} ` : '第三方'}登录，等待授权回调。`
  if (event.action === 'user.third_party.start') return `已发起${provider ? ` ${provider} ` : '第三方'}绑定授权，尚未完成绑定。`
  if (event.action === 'user.third_party.callback') return `${provider ? `${provider} ` : ''}绑定预览已准备，等待确认。`
  if (event.action === 'user.third_party.bind' && completed.includes('binding_created')) return `${provider ? `${provider} ` : ''}第三方绑定已完成。`
  if (event.action.startsWith('auth.login.') && completed.includes('session_created')) return `已通过${known(authMethods, details.auth_method) || provider || title.replace('登录', '')}完成登录，登录会话已创建。`
  if (event.action === 'oauth.authorize' && completed.includes('authorization_code_issued')) {
    const name = auditApplicationName(event)
    if (!name) return '已完成应用授权。'
    return [
      { type: 'text', text: '已为应用 ' },
      { type: 'application', text: name, logo: event.application?.logo_url || '', href: applicationHomepage(event.application?.homepage_url) },
      { type: 'text', text: ' 完成账号授权。' }
    ]
  }
  const changed = (details.changed_fields || []).map(field => known(fields, field)).filter(Boolean)
  return `${title}操作成功${changed.length ? `，涉及字段：${changed.join('、')}` : ''}。`
}

export function auditFullTime(value) {
  const date = new Date(value)
  if (!value || Number.isNaN(date.getTime())) return auditCopy.placeholder
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false, timeZoneName: 'short' }).format(date)
}

export function auditRelativeTime(value, now = Date.now()) {
  const date = new Date(value).getTime()
  if (!value || !Number.isFinite(date)) return auditCopy.placeholder
  const seconds = Math.max(0, Math.floor((now - date) / 1000))
  if (seconds < 60) return '刚刚'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`
  return `${Math.floor(seconds / 86400)} 天前`
}

export function auditDetailRows(event) {
  const details = event.details || {}
  const list = (values, map) => (Array.isArray(values) ? values : []).map(value => known(map, value) || value).join('、')
  return [
    ['事件 ID', event.id], ['发生时间', auditFullTime(event.occurred_at)], ['操作', known(auditActions, event.action) || '未知操作'],
    ['操作结果', auditOutcomeLabel(event.outcome)], ['结果说明', auditReasonLabel(event.reason_code)], ['原因码', event.reason_code],
    ['接口', [event.method, event.route].filter(Boolean).join(' ')], ['HTTP 状态', event.http_status], ['耗时', Number.isFinite(event.duration_ms) ? `${event.duration_ms} ms` : null],
    ['目标类型', known(targets, event.target_type) || event.target_type], ['目标 ID', event.target_id], ['应用 ID', event.client_id], ['应用名称（当前）', event.application?.name],
    ['来源 IP', event.ip], ['设备', event.device_label], ['认证方式', known(authMethods, details.auth_method) || details.auth_method],
    ['第三方平台', known(providers, details.provider) || details.provider], ['邮箱（已脱敏）', details.email_masked],
    ['变更字段', list(details.changed_fields, fields)], ['已完成步骤', list(details.completed_steps, steps)]
  ].map(([label, value]) => ({ label, value: valueOrDash(value) }))
}

export function localDateInput(now = Date.now()) {
  const date = new Date(now)
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

export function auditDateBounds(now = Date.now()) {
  return { min: localDateInput(now - 30 * dayMS), max: localDateInput(now) }
}

export function buildAuditQuery(filters, now = Date.now()) {
  const q = String(filters.q || '').trim()
  if ([...q].length > 100) throw new Error(auditCopy.longSearch)
  let from = now - (filters.period === '7' ? 7 : 30) * dayMS
  let to = now
  if (filters.period === 'custom') {
    const bounds = auditDateBounds(now)
    const { startDate, endDate } = filters
    if (!/^\d{4}-\d{2}-\d{2}$/.test(startDate || '') || !/^\d{4}-\d{2}-\d{2}$/.test(endDate || '') || startDate < bounds.min || endDate > bounds.max || startDate > endDate) throw new Error(auditCopy.invalidDates)
    const start = new Date(`${startDate}T00:00:00`)
    const end = new Date(`${endDate}T00:00:00`)
    if (localDateInput(start) !== startDate || localDateInput(end) !== endDate) throw new Error(auditCopy.invalidDates)
    end.setDate(end.getDate() + 1)
    from = Math.max(now - 30 * dayMS, start.getTime())
    to = Math.min(now, end.getTime())
    if (from >= to) throw new Error(auditCopy.invalidDates)
  }
  return { q, actions: auditActionMatches(q).join(','), outcome: filters.outcome || '', from: new Date(from).toISOString(), to: new Date(to).toISOString(), limit: 30 }
}

export function auditLoadError(error) {
  return error?.status === 400 ? auditCopy.invalidQuery : auditCopy.loadFailed
}

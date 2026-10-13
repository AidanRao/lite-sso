// Owns one browser-bound QR request and serializes its polling lifecycle.
export const createQRLoginController = ({ api, render, onChange, onComplete,
  now = Date.now, schedule = setTimeout, cancelTimer = clearTimeout }) => {
  let generation = 0
  let active = null
  let pollTimer = null
  let countdownTimer = null
  let state = { image: '', message: '', seconds: 0, loading: false, expired: false }
  const publish = patch => { state = { ...state, ...patch }; onChange(state) }
  const clearTimers = () => {
    cancelTimer(pollTimer)
    cancelTimer(countdownTimer)
    pollTimer = countdownTimer = null
  }
  const current = (version, request) => generation === version && active === request
  const terminal = (message, expired = false) => {
    ++generation
    clearTimers()
    publish({ image: '', message, seconds: 0, loading: false, expired })
  }
  const cancel = request => api.cancelQRCode(request.code, request.browser_token)
  const countdown = (version, request) => {
    if (!current(version, request)) return
    const seconds = Math.max(0, Math.ceil((Date.parse(request.expires_at) - now()) / 1000))
    publish({ seconds })
    if (!seconds) { terminal('二维码已过期', true); return }
    countdownTimer = schedule(() => countdown(version, request), 1000)
  }
  const poll = async (version, request) => {
    if (!current(version, request)) return
    try {
      const response = await api.checkQRCode(request.code, request.browser_token)
      if (!current(version, request)) return
      const data = response.data || response
      if (data.status === 'cancelled') { terminal('登录已取消，请刷新二维码'); return }
      if (data.status === 'consumed') { terminal('二维码已使用，请刷新'); return }
      if (data.status === 'scanned') publish({ message: '已扫描，请在 App 中确认' })
      if (data.status === 'confirmed') {
        clearTimers()
        publish({ message: '正在完成登录…', loading: true })
        try {
          const result = await api.completeQRCode({ code: request.code, login_ticket: data.login_ticket }, request.browser_token)
          if (!current(version, request)) return
          active = null
          publish({ message: '登录成功', loading: false, seconds: 0 })
          onComplete(result)
        } catch (error) {
          if (current(version, request)) terminal(error.message || '登录失败，请重新生成二维码')
        }
        return
      }
      publish({ message: data.status === 'scanned' ? '已扫描，请在 App 中确认' : '请使用 BUAA ClassHopper 扫码' })
    } catch (error) {
      if (!current(version, request)) return
      if ([400, 403, 409, 410].includes(error.status)) {
        terminal(error.status === 410 ? '二维码已过期' : error.message, error.status === 410)
        return
      }
      publish({ message: '连接暂时中断，正在重试…' })
    }
    if (current(version, request) && state.seconds > 0) pollTimer = schedule(() => poll(version, request), 2000)
  }
  return {
    async start(redirect) {
      const version = ++generation
      clearTimers()
      publish({ image: '', seconds: 0, loading: true, message: '正在生成二维码…', expired: false })
      try {
        if (active) {
          try { await cancel(active) } catch (error) { if (![409, 410].includes(error.status)) throw error }
          if (generation !== version) return
          active = null
        }
        const response = await api.getQRCode(redirect)
        const request = response.data || response
        if (generation !== version) { await cancel(request).catch(() => {}); return }
        active = request
        const image = await render(request.qr_payload)
        if (!current(version, request)) return
        publish({ image, loading: false, message: '请使用 BUAA ClassHopper 扫码' })
        countdown(version, request)
        if (state.seconds > 0) void poll(version, request)
      } catch (error) {
        if (generation === version) terminal(error.message || '获取二维码失败，请刷新')
      }
    },
    stop() {
      ++generation
      clearTimers()
      const request = active
      active = null
      if (request) void cancel(request).catch(() => {})
      publish({ image: '', message: '', seconds: 0, loading: false, expired: false })
    }
  }
}

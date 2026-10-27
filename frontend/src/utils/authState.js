import { computed, onScopeDispose, ref, toValue, watch } from 'vue'

// Timers use deadlines so background tabs do not extend a server cooldown.
export const useDeadline = ({ now = Date.now, schedule = setTimeout, cancel = clearTimeout } = {}) => {
  const seconds = ref(0)
  let deadline = 0
  let timer
  const stop = () => {
    cancel(timer)
    timer = undefined
    deadline = 0
    seconds.value = 0
  }
  const tick = () => {
    seconds.value = Math.max(0, Math.ceil((deadline - now()) / 1000))
    if (seconds.value) timer = schedule(tick, Math.min(1000, deadline - now()))
    else timer = undefined
  }
  const start = duration => {
    stop()
    deadline = now() + Math.max(0, Number(duration) || 0) * 1000
    tick()
  }
  onScopeDispose(stop)
  return { seconds, start, stop }
}

export const normalizeAuthEmail = email => String(email || '').trim().toLowerCase()
export const validAuthEmail = email => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(normalizeAuthEmail(email))

export const useEmailChallenge = (email, timers) => {
  const code = ref('')
  const challengeId = ref('')
  const visible = ref(false)
  const cooldown = useDeadline(timers)
  const expiry = useDeadline(timers)
  const clear = () => {
    challengeId.value = ''
    code.value = ''
    visible.value = false
    cooldown.stop()
    expiry.stop()
  }
  watch(() => normalizeAuthEmail(toValue(email)), clear, { flush: 'sync' })
  watch(expiry.seconds, seconds => {
    if (!seconds) {
      challengeId.value = ''
      code.value = ''
    }
  }, { flush: 'sync' })
  const accept = data => {
    if (!visible.value || !data.challenge_id) return
    code.value = ''
    cooldown.start(data.resend_after)
    expiry.start(data.expires_in)
    challengeId.value = data.challenge_id
    visible.value = false
  }
  return { code, challengeId, visible, countdown: cooldown.seconds, accept,
    valid: computed(() => Boolean(challengeId.value) && expiry.seconds.value > 0) }
}

// Acquire before asynchronous validation; keep the gate closed after navigation succeeds.
export const useAuthSubmission = () => {
  const busy = ref(false)
  let disposed = false
  onScopeDispose(() => { disposed = true })
  const run = async task => {
    if (busy.value || disposed) return
    busy.value = true
    let completed = false
    try { completed = await task(() => !disposed) === true }
    finally { if (!completed) busy.value = false }
  }
  return { busy, run }
}

// A closed or replaced dialog cannot publish responses into a newer challenge.
export const useCodeDelivery = ({ context, api, onSuccess, onClose }) => {
  const image = ref('')
  const captchaId = ref('')
  const captcha = ref('')
  const error = ref('')
  const loading = ref(false)
  const fetching = ref(false)
  let generation = 0
  const invalidate = () => {
    generation++
    image.value = captchaId.value = captcha.value = ''
    loading.value = fetching.value = false
  }
  const load = async () => {
    if (loading.value || !context().visible) return
    const version = ++generation
    fetching.value = true
    image.value = captchaId.value = captcha.value = ''
    try {
      const response = await api.getCaptcha()
      if (version !== generation) return
      const data = response.data || response
      if (!data.captcha_png_base64?.trim() || !data.captcha_id) throw new Error('验证码图片为空，请重试')
      image.value = data.captcha_png_base64
      captchaId.value = data.captcha_id
    } catch (cause) {
      if (version === generation) error.value = cause.message || '获取图形验证码失败'
    } finally {
      if (version === generation) fetching.value = false
    }
  }
  const close = () => { invalidate(); onClose() }
  const submit = async () => {
    if (loading.value || fetching.value || !context().visible) return
    if (!validAuthEmail(context().email)) { error.value = '请输入正确的邮箱地址'; return }
    if (!captchaId.value || !/^\d{4}$/.test(captcha.value)) { error.value = '请输入四位图形验证码'; return }
    const version = generation
    const request = { email: normalizeAuthEmail(context().email), purpose: context().purpose,
      captcha_id: captchaId.value, captcha: captcha.value }
    loading.value = true
    error.value = ''
    try {
      const response = await api.sendEmailCode(request)
      if (version !== generation) return
      onSuccess(response.data || response)
      close()
    } catch (cause) {
      if (version !== generation) return
      error.value = cause.message || '发送失败，请重试'
      loading.value = false
      await load()
    } finally {
      if (version === generation) loading.value = false
    }
  }
  watch(() => [context().visible, normalizeAuthEmail(context().email), context().purpose], () => {
    invalidate()
    error.value = ''
    if (context().visible) void load()
  }, { immediate: true, flush: 'sync' })
  onScopeDispose(invalidate)
  return { image, captcha, error, loading, fetching, load, close, submit }
}

export function submitGlobalLogout(redirectPath = '/login') {
  const form = document.createElement('form')
  const params = new URLSearchParams()

  if (redirectPath) {
    params.set('redirect', redirectPath)
  }

  form.method = 'POST'
  form.action = `/api/auth/logout${params.toString() ? `?${params.toString()}` : ''}`
  form.style.display = 'none'

  document.body.appendChild(form)
  form.submit()
}

export const getLoginRedirect = (route) => {
  if (route.query.redirect) {
    return route.query.redirect
  }

  const params = new URLSearchParams(window.location.search)
  if (params.has('client_id') || params.has('response_type') || params.has('redirect_uri')) {
    return `/oauth/authorize?${params.toString()}`
  }

  return '/profile'
}

export const getTargetClientID = (redirect) => {
  if (!redirect || redirect === '/profile') {
    return ''
  }

  try {
    const parsed = new URL(redirect, window.location.origin)
    if (parsed.pathname !== '/oauth/authorize') {
      return ''
    }
    return parsed.searchParams.get('client_id') || ''
  } catch (error) {
    return ''
  }
}

export const loadTargetClientName = async (redirect) => {
  const clientID = getTargetClientID(redirect)
  if (!clientID) {
    return ''
  }

  try {
    const response = await fetch(`/api/oauth/client?client_id=${encodeURIComponent(clientID)}`)
    if (!response.ok) {
      return ''
    }

    const result = await response.json()
    return result?.data?.name || ''
  } catch (error) {
    return ''
  }
}

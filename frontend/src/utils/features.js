// Missing decisions keep controlled navigation and pages unavailable.
export const featureEnabled = (features, key) => !key || features?.[key]?.enabled === true

export const filterFeatureNavigation = (items, features) => items.flatMap(item => {
  if (!featureEnabled(features, item.featureKey)) return []
  const result = { ...item, beta: Boolean(item.featureKey && features?.[item.featureKey]?.stage === 'beta') }
  if (item.children) {
    result.children = filterFeatureNavigation(item.children, features)
    if (!result.children.length) return []
  }
  return [result]
})

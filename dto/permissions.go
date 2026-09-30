package dto

// PermissionsResponse contains current-user access decisions, not release policies.
type PermissionsResponse struct {
	IsAdmin  bool                    `json:"is_admin"`
	Features map[string]FeatureState `json:"features"`
}

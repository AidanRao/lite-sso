package dto

// FeatureState is the evaluated release state for the current user.
type FeatureState struct {
	Enabled bool   `json:"enabled"`
	Stage   string `json:"stage"`
}

// FeatureUpdate contains a complete release policy.
type FeatureUpdate struct {
	Audience   string   `json:"audience"`
	Percentage *int     `json:"percentage"`
	Stage      string   `json:"stage"`
	UserIDs    []string `json:"user_ids"`
}

// FeatureResponse describes a registered feature and its release policy.
type FeatureResponse struct {
	Key        string   `json:"key"`
	Name       string   `json:"name"`
	Audience   string   `json:"audience"`
	Percentage int      `json:"percentage"`
	Stage      string   `json:"stage"`
	UserIDs    []string `json:"user_ids"`
}

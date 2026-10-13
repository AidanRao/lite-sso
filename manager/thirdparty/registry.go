// Package thirdparty registers external login providers and their protocol adapters.
package thirdparty

import (
	"context"
	"fmt"

	"sso-server/conf"
)

const (
	GitHub = "github"
	Feishu = "feishu"
	QQ     = "qq"
)

// Profile is the verified identity returned by an external provider.
type Profile struct {
	Provider    string `json:"provider"`
	ProviderUID string `json:"provider_uid"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	AvatarURL   string `json:"avatar_url"`
}

// Provider exchanges an authorization code for a verified external identity.
type Provider interface {
	Configured() bool
	AuthCodeURL(state string) string
	FetchProfile(ctx context.Context, code string) (*Profile, error)
}

type registration struct {
	name       string
	authMethod string
	provider   Provider
}

// Registry is a read-only catalog after its registrations are completed.
type Registry struct {
	ordered []registration
	byName  map[string]registration
	frozen  bool
}

// NewRegistry creates an empty registry for explicit startup registration.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]registration)}
}

// NewBuiltins registers the project's built-in external login providers.
func NewBuiltins(cfg *conf.Config) *Registry {
	if cfg == nil {
		cfg = &conf.Config{}
	}
	registry := NewRegistry()
	registry.mustRegister(GitHub, "GITHUB", newGitHubProvider(cfg.OAuth.GitHub))
	registry.mustRegister(Feishu, "FEISHU", newFeishuProvider(cfg.OAuth.Feishu))
	registry.mustRegister(QQ, "QQ", newQQProvider(cfg.OAuth.QQ))
	registry.Freeze()
	return registry
}

// Register adds one provider before the registry is frozen.
func (r *Registry) Register(name, authMethod string, provider Provider) error {
	if r == nil || r.frozen {
		return fmt.Errorf("third-party registry is frozen")
	}
	if name == "" || authMethod == "" || provider == nil {
		return fmt.Errorf("third-party registration is incomplete")
	}
	if _, exists := r.byName[name]; exists {
		return fmt.Errorf("third-party provider %q is already registered", name)
	}
	entry := registration{name: name, authMethod: authMethod, provider: provider}
	r.ordered = append(r.ordered, entry)
	r.byName[name] = entry
	return nil
}

// Freeze prevents registrations after startup.
func (r *Registry) Freeze() {
	if r != nil {
		r.frozen = true
	}
}

// Names returns registered providers in registration order.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.ordered))
	for _, entry := range r.ordered {
		names = append(names, entry.name)
	}
	return names
}

// Get returns a registered protocol adapter.
func (r *Registry) Get(name string) (Provider, bool) {
	if r == nil {
		return nil, false
	}
	entry, ok := r.byName[name]
	return entry.provider, ok
}

// AuthMethod returns the session method belonging to a registered provider.
func (r *Registry) AuthMethod(name string) (string, bool) {
	if r == nil {
		return "", false
	}
	entry, ok := r.byName[name]
	return entry.authMethod, ok
}

// IsRegistered reports whether a provider name is registered.
func (r *Registry) IsRegistered(name string) bool {
	_, ok := r.Get(name)
	return ok
}

func (r *Registry) mustRegister(name, authMethod string, provider Provider) {
	if err := r.Register(name, authMethod, provider); err != nil {
		panic(err)
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

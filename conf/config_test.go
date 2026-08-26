package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_TrustProxyHeadersFromEnvironment(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "test.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: \"8080\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ENV", "test")
	t.Setenv("CONFIG_FILE", configPath)
	t.Setenv("SERVER_TRUST_PROXY_HEADERS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Server.TrustProxyHeaders {
		t.Fatal("expected trusted proxy headers enabled")
	}
}

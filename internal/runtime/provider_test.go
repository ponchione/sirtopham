package runtime

import (
	"strings"
	"testing"

	appconfig "github.com/ponchione/sodoryard/internal/config"
)

func TestBuildOpenAICompatibleProviderRequiresConfiguredAPIKeyEnv(t *testing.T) {
	const envName = "SODORYARD_TEST_OPENAI_KEY"
	t.Setenv(envName, "")

	_, err := BuildProvider("remote", appconfig.ProviderConfig{
		Type:          "openai-compatible",
		BaseURL:       "https://example.com/v1",
		APIKeyEnv:     envName,
		Model:         "test-model",
		ContextLength: 8192,
	})
	if err == nil || !strings.Contains(err.Error(), envName) {
		t.Fatalf("BuildProvider error = %v, want missing %s error", err, envName)
	}
}

func TestBuildOpenAICompatibleProviderAllowsKeylessWhenNoAPIKeyEnv(t *testing.T) {
	p, err := BuildProvider("local", appconfig.ProviderConfig{
		Type:          "openai-compatible",
		BaseURL:       "http://localhost:8080/v1",
		Model:         "test-model",
		ContextLength: 8192,
	})
	if err != nil {
		t.Fatalf("BuildProvider returned error: %v", err)
	}
	if p == nil {
		t.Fatal("BuildProvider returned nil provider")
	}
}


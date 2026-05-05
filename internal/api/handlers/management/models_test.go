package management

import (
	"context"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestOfficialCodexModelsForInstanceIncludesPaidUnion(t *testing.T) {
	manager := coreauth.NewManager(&memoryAuthStore{}, nil, nil)
	_, err := manager.Register(context.Background(), &coreauth.Auth{
		ID:       "paid-plus",
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"plan_type": "plus",
		},
	})
	if err != nil {
		t.Fatalf("register auth: %v", err)
	}

	h := NewHandler(&config.Config{}, "", manager)
	models := h.officialCodexModelsForInstance()

	if !containsModelID(models, "gpt-5.3-codex-spark") {
		t.Fatal("expected paid official Codex catalog to include gpt-5.3-codex-spark")
	}
	if containsModelID(models, "gpt-image-2") {
		t.Fatal("expected management catalog to exclude runtime built-in gpt-image-2")
	}
}

func TestOfficialCodexModelsForInstanceUsesEnvTiers(t *testing.T) {
	t.Setenv("CPA_CODEX_CATALOG_TIERS", "codex-plus,codex-pro,codex-team")

	h := NewHandler(&config.Config{}, "", nil)
	models := h.officialCodexModelsForInstance()

	if !containsModelID(models, "gpt-5.3-codex-spark") {
		t.Fatal("expected paid env tier union to include gpt-5.3-codex-spark")
	}
	if containsModelID(models, "gpt-image-2") {
		t.Fatal("expected env tier catalog to exclude runtime built-in gpt-image-2")
	}
}

func TestOfficialCodexModelsForInstanceFreeDoesNotIncludeSpark(t *testing.T) {
	manager := coreauth.NewManager(&memoryAuthStore{}, nil, nil)
	_, err := manager.Register(context.Background(), &coreauth.Auth{
		ID:       "free",
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"plan_type": "free",
		},
	})
	if err != nil {
		t.Fatalf("register auth: %v", err)
	}

	h := NewHandler(&config.Config{}, "", manager)
	models := h.officialCodexModelsForInstance()

	if containsModelID(models, "gpt-5.3-codex-spark") {
		t.Fatal("expected free official Codex catalog to exclude gpt-5.3-codex-spark")
	}
}

func containsModelID(models []*registry.ModelInfo, id string) bool {
	for _, model := range models {
		if model != nil && model.ID == id {
			return true
		}
	}
	return false
}

package management

import (
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
)

// GetAvailableModels returns the official static Codex model catalog visible to
// this CPA instance. It intentionally does not call /v1/models or
// registry.GetAvailableModels, because those are runtime account availability
// views and must not become the source of truth for management catalog display.
func (h *Handler) GetAvailableModels(c *gin.Context) {
	data := modelsToManagementEntries(h.officialCodexModelsForInstance())

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
		"models": data,
	})
}

func (h *Handler) officialCodexModelsForInstance() []*registry.ModelInfo {
	if models, ok := officialCodexModelsFromEnv(); ok {
		return models
	}

	if h == nil || h.authManager == nil {
		return registry.GetCodexProCatalogModels()
	}

	auths := h.authManager.List()
	includeFree := false
	includePaid := false

	for _, auth := range auths {
		if auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
			continue
		}
		planType := ""
		if auth.Attributes != nil {
			planType = strings.ToLower(strings.TrimSpace(auth.Attributes["plan_type"]))
		}
		switch planType {
		case "free":
			includeFree = true
		case "plus", "pro", "team", "business", "go":
			includePaid = true
		default:
			includePaid = true
		}
	}

	if !includeFree && !includePaid {
		return registry.GetCodexProCatalogModels()
	}

	var models []*registry.ModelInfo
	if includeFree {
		models = append(models, registry.GetCodexFreeCatalogModels()...)
	}
	if includePaid {
		models = append(models, registry.GetCodexPlusCatalogModels()...)
		models = append(models, registry.GetCodexProCatalogModels()...)
		models = append(models, registry.GetCodexTeamCatalogModels()...)
	}
	return dedupeModelInfosByID(models)
}

func officialCodexModelsFromEnv() ([]*registry.ModelInfo, bool) {
	raw := strings.TrimSpace(os.Getenv("CPA_CODEX_CATALOG_TIERS"))
	if raw == "" {
		return nil, false
	}

	var models []*registry.ModelInfo
	for _, part := range strings.Split(raw, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "codex-free", "free":
			models = append(models, registry.GetCodexFreeCatalogModels()...)
		case "codex-plus", "plus":
			models = append(models, registry.GetCodexPlusCatalogModels()...)
		case "codex-pro", "pro":
			models = append(models, registry.GetCodexProCatalogModels()...)
		case "codex-team", "team", "business", "go":
			models = append(models, registry.GetCodexTeamCatalogModels()...)
		}
	}
	if len(models) == 0 {
		return registry.GetCodexProCatalogModels(), true
	}
	return dedupeModelInfosByID(models), true
}

func dedupeModelInfosByID(models []*registry.ModelInfo) []*registry.ModelInfo {
	seen := make(map[string]*registry.ModelInfo, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, exists := seen[key]; !exists {
			seen[key] = model
		}
	}

	out := make([]*registry.ModelInfo, 0, len(seen))
	for _, model := range seen {
		out = append(out, model)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func modelsToManagementEntries(models []*registry.ModelInfo) []gin.H {
	data := make([]gin.H, 0, len(models))
	for _, model := range models {
		if model == nil || strings.TrimSpace(model.ID) == "" {
			continue
		}
		entry := gin.H{
			"id":     model.ID,
			"object": "model",
		}
		if model.Created != 0 {
			entry["created"] = model.Created
		}
		if model.OwnedBy != "" {
			entry["owned_by"] = model.OwnedBy
		}
		if model.DisplayName != "" {
			entry["display_name"] = model.DisplayName
		}
		data = append(data, entry)
	}
	return data
}

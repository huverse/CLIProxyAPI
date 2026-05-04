package management

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
)

// GetAvailableModels returns the OpenAI-compatible model list through the
// management API, protected by the management middleware.
func (h *Handler) GetAvailableModels(c *gin.Context) {
	models := registry.GetGlobalRegistry().GetAvailableModels("openai")
	data := make([]gin.H, 0, len(models))

	for _, model := range models {
		if model == nil {
			continue
		}
		id, _ := model["id"].(string)
		if id == "" {
			continue
		}

		entry := gin.H{
			"id":     id,
			"object": "model",
		}
		if created, ok := model["created"]; ok {
			entry["created"] = created
		}
		if ownedBy, ok := model["owned_by"]; ok {
			entry["owned_by"] = ownedBy
		}
		if displayName, ok := model["display_name"]; ok {
			entry["display_name"] = displayName
		}
		data = append(data, entry)
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
		"models": data,
	})
}

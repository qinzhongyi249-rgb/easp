package handlers

import (
	"log"
	"net/http"

	"github.com/easp-platform/easp/internal/models"
	"github.com/easp-platform/easp/internal/repositories"
	"github.com/gin-gonic/gin"
)

// ModelConfigHandler 模型配置处理器
type ModelConfigHandler struct {
	providerRepo *repositories.ModelProviderRepository
	configRepo   *repositories.ModelConfigRepository
}

func NewModelConfigHandler() *ModelConfigHandler {
	return &ModelConfigHandler{
		providerRepo: repositories.NewModelProviderRepository(),
		configRepo:   repositories.NewModelConfigRepository(),
	}
}

// ========== 模型提供商 API ==========

// CreateProvider 创建提供商
func (h *ModelConfigHandler) CreateProvider(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var provider models.ModelProvider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	provider.TenantID = tenantID

	if err := h.providerRepo.Create(&provider); err != nil {
		log.Printf("Failed to create provider: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create provider", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, provider)
}

// GetProvider 获取提供商
func (h *ModelConfigHandler) GetProvider(c *gin.Context) {
	providerID := c.Param("providerId")
	provider, err := h.providerRepo.GetByID(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}
	c.JSON(http.StatusOK, provider)
}

// ListProviders 列出提供商
func (h *ModelConfigHandler) ListProviders(c *gin.Context) {
	tenantID := c.Param("tenantId")
	
	// 可选参数：只返回启用的
	enabledOnly := c.Query("enabled")
	
	var providers []models.ModelProvider
	var err error
	
	if enabledOnly == "true" {
		providers, err = h.providerRepo.ListEnabled(tenantID)
	} else {
		providers, err = h.providerRepo.ListByTenant(tenantID)
	}
	
	if err != nil {
		log.Printf("Failed to list providers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list providers", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, providers)
}

// UpdateProvider 更新提供商
func (h *ModelConfigHandler) UpdateProvider(c *gin.Context) {
	providerID := c.Param("providerId")
	provider, err := h.providerRepo.GetByID(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}

	if err := c.ShouldBindJSON(provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.providerRepo.Update(provider); err != nil {
		log.Printf("Failed to update provider: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update provider", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// DeleteProvider 删除提供商
func (h *ModelConfigHandler) DeleteProvider(c *gin.Context) {
	providerID := c.Param("providerId")
	if err := h.providerRepo.Delete(providerID); err != nil {
		log.Printf("Failed to delete provider: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete provider", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// ========== 模型配置 API ==========

// CreateConfig 创建模型配置
func (h *ModelConfigHandler) CreateConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")
	var config models.ModelConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config.TenantID = tenantID

	if err := h.configRepo.Create(&config); err != nil {
		log.Printf("Failed to create model config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create model config", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, config)
}

// GetConfig 获取模型配置
func (h *ModelConfigHandler) GetConfig(c *gin.Context) {
	configID := c.Param("configId")
	config, err := h.configRepo.GetByID(configID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model config not found"})
		return
	}
	c.JSON(http.StatusOK, config)
}

// ListConfigs 列出模型配置
func (h *ModelConfigHandler) ListConfigs(c *gin.Context) {
	tenantID := c.Param("tenantId")
	
	// 可选参数
	enabledOnly := c.Query("enabled")
	providerID := c.Query("provider_id")
	
	var configs []models.ModelConfig
	var err error
	
	if providerID != "" {
		configs, err = h.configRepo.ListByProvider(providerID)
	} else if enabledOnly == "true" {
		configs, err = h.configRepo.ListEnabled(tenantID)
	} else {
		configs, err = h.configRepo.ListByTenant(tenantID)
	}
	
	if err != nil {
		log.Printf("Failed to list model configs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list model configs", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, configs)
}

// UpdateConfig 更新模型配置
func (h *ModelConfigHandler) UpdateConfig(c *gin.Context) {
	configID := c.Param("configId")
	config, err := h.configRepo.GetByID(configID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model config not found"})
		return
	}

	if err := c.ShouldBindJSON(config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.configRepo.Update(config); err != nil {
		log.Printf("Failed to update model config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update model config", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// DeleteConfig 删除模型配置
func (h *ModelConfigHandler) DeleteConfig(c *gin.Context) {
	configID := c.Param("configId")
	if err := h.configRepo.Delete(configID); err != nil {
		log.Printf("Failed to delete model config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete model config", "details": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// SetDefaultConfig 设置默认模型
func (h *ModelConfigHandler) SetDefaultConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")
	configID := c.Param("configId")
	
	if err := h.configRepo.SetDefault(configID, tenantID); err != nil {
		log.Printf("Failed to set default model: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default model", "details": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Default model set successfully"})
}

// GetDefaultConfig 获取默认模型配置
func (h *ModelConfigHandler) GetDefaultConfig(c *gin.Context) {
	tenantID := c.Param("tenantId")
	
	config, err := h.configRepo.GetDefault(tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No default model configured"})
		return
	}
	
	c.JSON(http.StatusOK, config)
}

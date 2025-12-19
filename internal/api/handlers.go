package api

import (
	"net/http"
	"strconv"

	"github.com/ety001/sps-fund-watcher/internal/storage"
	"github.com/gin-gonic/gin"
)

// Handler handles API requests
type Handler struct {
	storage *storage.MongoDB
}

// NewHandler creates a new API handler
func NewHandler(storage *storage.MongoDB) *Handler {
	return &Handler{
		storage: storage,
	}
}

// GetOperations handles GET /api/v1/accounts/:account/operations
func (h *Handler) GetOperations(c *gin.Context) {
	account := c.Param("account")
	opType := c.Query("type") // Optional filter by operation type

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	ctx := c.Request.Context()
	result, err := h.storage.GetOperations(ctx, account, opType, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetTransfers handles GET /api/v1/accounts/:account/transfers
func (h *Handler) GetTransfers(c *gin.Context) {
	account := c.Param("account")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	ctx := c.Request.Context()
	result, err := h.storage.GetOperations(ctx, account, "transfer", page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetUpdates handles GET /api/v1/accounts/:account/updates
func (h *Handler) GetUpdates(c *gin.Context) {
	account := c.Param("account")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	ctx := c.Request.Context()
	
	// Get account_update and account_update2 operations
	result1, err := h.storage.GetOperations(ctx, account, "account_update", page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result2, err := h.storage.GetOperations(ctx, account, "account_update2", page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Combine results
	combined := gin.H{
		"operations": append(result1.Operations, result2.Operations...),
		"total":      result1.Total + result2.Total,
		"page":       page,
		"page_size":  pageSize,
		"has_more":   result1.HasMore || result2.HasMore,
	}

	c.JSON(http.StatusOK, combined)
}

// GetAccounts handles GET /api/v1/accounts
func (h *Handler) GetAccounts(c *gin.Context) {
	ctx := c.Request.Context()
	accounts, err := h.storage.GetTrackedAccounts(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

// Health handles GET /api/v1/health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}


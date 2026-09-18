package database_instances

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	users_middleware "databasus-backend/internal/features/users/middleware"
)

type DatabaseInstanceController struct {
	instanceService *DatabaseInstanceService
}

func (c *DatabaseInstanceController) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/database-instances/create", c.RegisterInstance)
	router.POST("/database-instances/update", c.UpdateInstance)
	router.GET("/database-instances", c.GetInstances)
	router.GET("/database-instances/:id", c.GetInstance)
	router.DELETE("/database-instances/:id", c.DeleteInstance)
	router.POST("/database-instances/:id/discover", c.DiscoverDatabases)
	router.POST("/database-instances/bulk-configure-backups", c.BulkConfigureBackups)
}

func (c *DatabaseInstanceController) RegisterPublicRoutes(_ *gin.RouterGroup) {
}

// RegisterInstance
// @Summary Register a database instance
// @Description Register a database server instance, test connectivity, and save it
// @Tags database-instances
// @Accept json
// @Produce json
// @Param request body DatabaseInstance true "Instance registration data with workspaceId"
// @Success 201 {object} DatabaseInstance
// @Failure 400
// @Failure 401
// @Router /database-instances/create [post]
func (c *DatabaseInstanceController) RegisterInstance(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request DatabaseInstance
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.WorkspaceID == uuid.Nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "workspaceId is required"})
		return
	}

	instance, err := c.instanceService.RegisterInstance(ctx.Request.Context(), user, request.WorkspaceID, &request)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	instance.HideSensitiveData()

	ctx.JSON(http.StatusCreated, instance)
}

// UpdateInstance
// @Summary Update a database instance
// @Description Update a registered database instance and re-test connectivity
// @Tags database-instances
// @Accept json
// @Produce json
// @Param request body DatabaseInstance true "Instance update data"
// @Success 200 {object} DatabaseInstance
// @Failure 400
// @Failure 401
// @Router /database-instances/update [post]
func (c *DatabaseInstanceController) UpdateInstance(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request DatabaseInstance
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.ID == uuid.Nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	instance, err := c.instanceService.UpdateInstance(ctx.Request.Context(), user, &request)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	instance.HideSensitiveData()

	ctx.JSON(http.StatusOK, instance)
}

// GetInstances
// @Summary List database instances
// @Description List all database instances in a workspace
// @Tags database-instances
// @Produce json
// @Param workspaceId query string true "Workspace ID"
// @Success 200 {array} DatabaseInstance
// @Failure 400
// @Failure 401
// @Router /database-instances [get]
func (c *DatabaseInstanceController) GetInstances(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	workspaceID, err := uuid.Parse(ctx.Query("workspaceId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid workspaceId"})
		return
	}

	instances, err := c.instanceService.GetInstancesByWorkspace(ctx.Request.Context(), user, workspaceID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, instances)
}

// GetInstance
// @Summary Get a database instance
// @Description Get a database instance by ID
// @Tags database-instances
// @Produce json
// @Param id path string true "Instance ID"
// @Success 200 {object} DatabaseInstance
// @Failure 400
// @Failure 401
// @Router /database-instances/{id} [get]
func (c *DatabaseInstanceController) GetInstance(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance ID"})
		return
	}

	instance, err := c.instanceService.GetInstance(ctx.Request.Context(), user, id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, instance)
}

// DeleteInstance
// @Summary Delete a database instance
// @Description Delete a registered database instance
// @Tags database-instances
// @Param id path string true "Instance ID"
// @Success 204
// @Failure 400
// @Failure 401
// @Router /database-instances/{id} [delete]
func (c *DatabaseInstanceController) DeleteInstance(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance ID"})
		return
	}

	if err := c.instanceService.DeleteInstance(ctx.Request.Context(), user, id); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DiscoverDatabases
// @Summary Discover databases on an instance
// @Description Scan a registered instance and return all accessible databases
// @Tags database-instances
// @Produce json
// @Param id path string true "Instance ID"
// @Success 200 {object} DiscoverDatabasesResponse
// @Failure 400
// @Failure 401
// @Router /database-instances/{id}/discover [post]
func (c *DatabaseInstanceController) DiscoverDatabases(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid instance ID"})
		return
	}

	response, err := c.instanceService.DiscoverDatabases(ctx.Request.Context(), user, id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// BulkConfigureBackups
// @Summary Bulk configure backups for discovered databases
// @Description Create a managed database and a backup config for every selected discovered database on an instance
// @Tags database-instances
// @Accept json
// @Produce json
// @Param request body BulkConfigureBackupsRequest true "Instance ID, database names, and the backup config template"
// @Success 200 {object} BulkConfigureBackupsResponse
// @Failure 400
// @Failure 401
// @Router /database-instances/bulk-configure-backups [post]
func (c *DatabaseInstanceController) BulkConfigureBackups(ctx *gin.Context) {
	user, ok := users_middleware.GetUserFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request BulkConfigureBackupsRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.InstanceID == uuid.Nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "instanceId is required"})
		return
	}

	if len(request.DatabaseNames) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "databaseNames is required"})
		return
	}

	response, err := c.instanceService.BulkConfigureBackups(ctx.Request.Context(), user, &request)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

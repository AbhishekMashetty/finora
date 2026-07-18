package handler

import (
	"net/http"
	"time"

	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// GoalHandler exposes savings-goal CRUD endpoints.
type GoalHandler struct {
	service domain.GoalService
}

// NewGoalHandler builds a GoalHandler backed by service.
func NewGoalHandler(service domain.GoalService) *GoalHandler {
	return &GoalHandler{service: service}
}

type createGoalRequest struct {
	Name         string  `json:"name" binding:"required"`
	TargetAmount float64 `json:"target_amount" binding:"required"`
	TargetDate   string  `json:"target_date" binding:"required"`
}

// updateGoalRequest is a full-replace PUT body, matching budget's Update
// convention. CurrentAmount has no `required` tag (unlike the other fields)
// because 0 is a legitimate value — e.g. explicitly resetting progress —
// and go-playground/validator's `required` tag treats a numeric zero as
// "absent", which would wrongly reject that case.
type updateGoalRequest struct {
	Name          string  `json:"name" binding:"required"`
	TargetAmount  float64 `json:"target_amount" binding:"required"`
	TargetDate    string  `json:"target_date" binding:"required"`
	CurrentAmount float64 `json:"current_amount"`
}

// parseGoalDate parses a target_date field as RFC3339, falling back to the
// simpler YYYY-MM-DD form so clients can send either.
func parseGoalDate(v string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, v)
}

// List handles GET /api/v1/goals.
func (h *GoalHandler) List(c *gin.Context) {
	userID := middleware.UserID(c)

	goals, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "failed to list goals", nil)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"goals": goals})
}

// Create handles POST /api/v1/goals.
func (h *GoalHandler) Create(c *gin.Context) {
	userID := middleware.UserID(c)

	var req createGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", err.Error())
		return
	}

	targetDate, err := parseGoalDate(req.TargetDate)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "target_date must be an RFC3339 or YYYY-MM-DD date", nil)
		return
	}

	goal, err := h.service.Create(c.Request.Context(), userID, domain.CreateGoalInput{
		Name:         req.Name,
		TargetAmount: req.TargetAmount,
		TargetDate:   targetDate,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}

	httpx.Success(c, http.StatusCreated, gin.H{"goal": goal})
}

// Get handles GET /api/v1/goals/:id.
func (h *GoalHandler) Get(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	goal, err := h.service.Get(c.Request.Context(), userID, id)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"goal": goal})
}

// Update handles PUT /api/v1/goals/:id.
func (h *GoalHandler) Update(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	var req updateGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "invalid request body", err.Error())
		return
	}

	targetDate, err := parseGoalDate(req.TargetDate)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "target_date must be an RFC3339 or YYYY-MM-DD date", nil)
		return
	}

	goal, err := h.service.Update(c.Request.Context(), userID, id, domain.UpdateGoalInput{
		Name:          req.Name,
		TargetAmount:  req.TargetAmount,
		TargetDate:    targetDate,
		CurrentAmount: req.CurrentAmount,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"goal": goal})
}

// Delete handles DELETE /api/v1/goals/:id.
func (h *GoalHandler) Delete(c *gin.Context) {
	userID := middleware.UserID(c)
	id := c.Param("id")

	if err := h.service.Delete(c.Request.Context(), userID, id); err != nil {
		writeServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

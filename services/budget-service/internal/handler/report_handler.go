package handler

import (
	"net/http"
	"time"

	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/shared/httpx"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// ReportHandler exposes the reports endpoint.
type ReportHandler struct {
	service domain.ReportService
}

// NewReportHandler builds a ReportHandler backed by service.
func NewReportHandler(service domain.ReportService) *ReportHandler {
	return &ReportHandler{service: service}
}

// parseReportDate parses a from/to query value as RFC3339, falling back to
// the simpler YYYY-MM-DD form so clients can send either.
func parseReportDate(v string) (time.Time, bool) {
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// Summary handles GET /api/v1/reports/summary?from=...&to=....
//
// from/to are required — this is a real, computed budget-vs-actual report
// (see domain.ReportSummary), so there is no sane implicit default range to
// fall back to. Missing or unparseable values are a 400 VALIDATION_ERROR.
func (h *ReportHandler) Summary(c *gin.Context) {
	userID := middleware.UserID(c)

	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "from and to query parameters are required", nil)
		return
	}

	from, ok := parseReportDate(fromStr)
	if !ok {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "from must be an RFC3339 timestamp or YYYY-MM-DD date", gin.H{"field": "from"})
		return
	}
	to, ok := parseReportDate(toStr)
	if !ok {
		httpx.Fail(c, http.StatusBadRequest, httpx.CodeValidation, "to must be an RFC3339 timestamp or YYYY-MM-DD date", gin.H{"field": "to"})
		return
	}

	summary, err := h.service.Summary(c.Request.Context(), userID, from, to)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, httpx.CodeInternal, "failed to build report summary", nil)
		return
	}

	httpx.Success(c, http.StatusOK, gin.H{"summary": summary})
}

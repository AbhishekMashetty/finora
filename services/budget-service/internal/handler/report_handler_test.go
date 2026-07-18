package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/finora/budget-service/internal/domain"
	"github.com/finora/shared/middleware"
	"github.com/gin-gonic/gin"
)

// fakeReportService is a hand-written fake of domain.ReportService.
type fakeReportService struct {
	summaryFn func(ctx context.Context, userID string, from, to time.Time) (*domain.ReportSummary, error)
}

func (f *fakeReportService) Summary(ctx context.Context, userID string, from, to time.Time) (*domain.ReportSummary, error) {
	return f.summaryFn(ctx, userID, from, to)
}

func newReportTestRouter(svc domain.ReportService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := NewReportHandler(svc)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.RequireIdentity())
	{
		reports := v1.Group("/reports")
		reports.GET("/summary", h.Summary)
	}

	return r
}

// TestReportHandler_Summary_ComputesRealFigures confirms the reports
// endpoint returns the real, computed budget-vs-actual shape (not the old
// Phase-3 placeholder echo).
func TestReportHandler_Summary_ComputesRealFigures(t *testing.T) {
	svc := &fakeReportService{
		summaryFn: func(_ context.Context, _ string, from, to time.Time) (*domain.ReportSummary, error) {
			return &domain.ReportSummary{
				From: from.Format(time.RFC3339),
				To:   to.Format(time.RFC3339),
				Categories: []domain.CategorySummary{
					{Category: "groceries", Period: domain.PeriodMonthly, Budgeted: 500, Actual: 320, Remaining: 180},
				},
				TotalBudgeted: 500,
				TotalActual:   320,
			}, nil
		},
	}
	r := newReportTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary?from=2026-01-01&to=2026-01-31", nil)
	req.Header.Set("X-User-Id", "user-1")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusOK, w.Body.String())
	}

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if !env.Success {
		t.Fatalf("success = %v, want true", env.Success)
	}

	var body struct {
		Summary domain.ReportSummary `json:"summary"`
	}
	if err := json.Unmarshal(env.Data, &body); err != nil {
		t.Fatalf("failed to decode data.summary: %v", err)
	}
	if len(body.Summary.Categories) != 1 {
		t.Fatalf("len(Categories) = %d, want 1", len(body.Summary.Categories))
	}
	if body.Summary.Categories[0].Remaining != 180 {
		t.Errorf("Remaining = %v, want 180", body.Summary.Categories[0].Remaining)
	}
	if body.Summary.TotalBudgeted != 500 || body.Summary.TotalActual != 320 {
		t.Errorf("totals = %v/%v, want 500/320", body.Summary.TotalBudgeted, body.Summary.TotalActual)
	}
}

// TestReportHandler_Summary_MissingUserIdIs401 confirms the reports endpoint
// is owner-scoped and requires identity.
func TestReportHandler_Summary_MissingUserIdIs401(t *testing.T) {
	svc := &fakeReportService{}
	r := newReportTestRouter(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary?from=2026-01-01&to=2026-01-31", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusUnauthorized, w.Body.String())
	}
}

// TestReportHandler_Summary_RequiresFromAndTo confirms from/to are required
// query params -- there is no implicit default range.
func TestReportHandler_Summary_RequiresFromAndTo(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "missing both", url: "/api/v1/reports/summary"},
		{name: "missing to", url: "/api/v1/reports/summary?from=2026-01-01"},
		{name: "missing from", url: "/api/v1/reports/summary?to=2026-01-31"},
		{name: "unparseable from", url: "/api/v1/reports/summary?from=not-a-date&to=2026-01-31"},
		{name: "unparseable to", url: "/api/v1/reports/summary?from=2026-01-01&to=not-a-date"},
		{name: "inverted range (to before from)", url: "/api/v1/reports/summary?from=2026-01-31&to=2026-01-01"},
	}

	svc := &fakeReportService{
		summaryFn: func(_ context.Context, _ string, from, to time.Time) (*domain.ReportSummary, error) {
			t.Fatal("service should never be called when from/to validation fails")
			return nil, nil
		},
	}
	r := newReportTestRouter(svc)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			req.Header.Set("X-User-Id", "user-1")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body.String())
			}

			var env envelope
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if env.Error == nil || env.Error.Code != "VALIDATION_ERROR" {
				t.Errorf("error code = %+v, want VALIDATION_ERROR", env.Error)
			}
		})
	}
}

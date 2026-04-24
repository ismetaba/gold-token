package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/reporting/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/reporting/internal/generator"
	"github.com/ismetaba/gold-token/backend/services/reporting/internal/repo"
)

type Handlers struct {
	jobs        repo.ReportJobRepo
	queries     repo.QueryRepo
	adminSecret string
	log         *zap.Logger
}

func NewHandlers(jobs repo.ReportJobRepo, queries repo.QueryRepo, adminSecret string, log *zap.Logger) *Handlers {
	return &Handlers{jobs: jobs, queries: queries, adminSecret: adminSecret, log: log}
}

func (h *Handlers) Routes(env string) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	if env == "local" {
		r.Use(httputil.CORSMiddleware(httputil.LocalCORSConfig()))
	} else {
		r.Use(httputil.CORSMiddleware(httputil.DefaultCORSConfig()))
	}

	rl := httputil.NewRateLimiter(60, time.Minute)
	r.Use(rl.Middleware)

	r.Get("/health", h.health)

	r.Route("/reports", func(r chi.Router) {
		r.Use(h.requireAdmin)
		r.Get("/transactions", h.transactions)
		r.Get("/reserves", h.reserves)
		r.Get("/compliance", h.compliance)
		r.Post("/generate", h.generate)
		r.Get("/{id}/status", h.jobStatus)
		r.Get("/{id}/download", h.download)
	})

	return r
}

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /reports/transactions?from=2026-01-01&to=2026-12-31
func (h *Handlers) transactions(w http.ResponseWriter, r *http.Request) {
	from, to := parseDateRange(r)
	summary, err := h.queries.TransactionSummary(r.Context(), from, to)
	if err != nil {
		h.log.Error("transaction summary", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.REPORTING.INTERNAL", "failed to generate report")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": summary, "from": from.Format("2006-01-02"), "to": to.Format("2006-01-02")})
}

// GET /reports/reserves?from=2026-01-01&to=2026-12-31
func (h *Handlers) reserves(w http.ResponseWriter, r *http.Request) {
	from, to := parseDateRange(r)
	summary, err := h.queries.ReserveSummary(r.Context(), from, to)
	if err != nil {
		h.log.Error("reserve summary", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.REPORTING.INTERNAL", "failed to generate report")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reserves": summary, "from": from.Format("2006-01-02"), "to": to.Format("2006-01-02")})
}

// GET /reports/compliance
func (h *Handlers) compliance(w http.ResponseWriter, r *http.Request) {
	summary, err := h.queries.ComplianceSummary(r.Context())
	if err != nil {
		h.log.Error("compliance summary", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.REPORTING.INTERNAL", "failed to generate report")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// POST /reports/generate — async report generation.
func (h *Handlers) generate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReportType string          `json:"report_type"`
		Parameters json.RawMessage `json:"parameters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.REPORTING.VALIDATION", "invalid request body")
		return
	}

	validTypes := map[string]bool{"transactions": true, "reserves": true, "compliance": true}
	if !validTypes[req.ReportType] {
		writeError(w, http.StatusBadRequest, "GOLD.REPORTING.VALIDATION", "report_type must be transactions, reserves, or compliance")
		return
	}

	now := time.Now().UTC()
	job := domain.ReportJob{
		ID:          uuid.Must(uuid.NewV7()),
		ReportType:  req.ReportType,
		Parameters:  req.Parameters,
		Status:      "pending",
		RequestedBy: "admin",
		CreatedAt:   now,
	}

	if err := h.jobs.Create(r.Context(), job); err != nil {
		h.log.Error("create report job", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.REPORTING.INTERNAL", "failed to create report job")
		return
	}

	// Run generation asynchronously.
	go h.runJob(job)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"job_id": job.ID.String(),
		"status": "pending",
	})
}

// runJob executes a report job in the background and updates its status.
func (h *Handlers) runJob(job domain.ReportJob) {
	ctx := context.Background()

	running := "running"
	_ = h.jobs.UpdateStatus(ctx, job.ID, running, "", nil)

	var params struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if len(job.Parameters) > 0 {
		_ = json.Unmarshal(job.Parameters, &params)
	}
	from, to := parseStoredDateRange(params.From, params.To)

	var genErr error
	switch job.ReportType {
	case "transactions":
		_, genErr = generator.TransactionCSV(ctx, h.queries, from, to)
	case "reserves":
		_, genErr = generator.ReserveCSV(ctx, h.queries, from, to)
	case "compliance":
		_, genErr = generator.ComplianceCSV(ctx, h.queries)
	}

	now := time.Now().UTC()
	if genErr != nil {
		h.log.Error("async report job failed", zap.String("job_id", job.ID.String()), zap.Error(genErr))
		_ = h.jobs.UpdateStatus(ctx, job.ID, "failed", genErr.Error(), &now)
		return
	}
	_ = h.jobs.UpdateStatus(ctx, job.ID, "completed", "", &now)
	h.log.Info("report job completed", zap.String("job_id", job.ID.String()), zap.String("type", job.ReportType))
}

// GET /reports/{id}/status
func (h *Handlers) jobStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.REPORTING.VALIDATION", "invalid id")
		return
	}

	job, err := h.jobs.ByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "GOLD.REPORTING.001", "report job not found")
			return
		}
		h.log.Error("get report job", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.REPORTING.INTERNAL", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":          job.ID.String(),
		"report_type": job.ReportType,
		"status":      job.Status,
		"error":       job.Error,
		"created_at":  job.CreatedAt.UTC().Format(time.RFC3339),
	})
}

// GET /reports/{id}/download — stream CSV for a completed report job.
func (h *Handlers) download(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.REPORTING.VALIDATION", "invalid id")
		return
	}

	job, err := h.jobs.ByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "GOLD.REPORTING.001", "report job not found")
			return
		}
		h.log.Error("get report job for download", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.REPORTING.INTERNAL", "internal error")
		return
	}
	if job.Status != "completed" {
		writeError(w, http.StatusConflict, "GOLD.REPORTING.002", fmt.Sprintf("report not ready: status=%s", job.Status))
		return
	}

	// Generate CSV on the fly using the stored parameters.
	var params struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if len(job.Parameters) > 0 {
		_ = json.Unmarshal(job.Parameters, &params)
	}
	from, to := parseStoredDateRange(params.From, params.To)

	var csvData []byte
	switch job.ReportType {
	case "transactions":
		csvData, err = generator.TransactionCSV(r.Context(), h.queries, from, to)
	case "reserves":
		csvData, err = generator.ReserveCSV(r.Context(), h.queries, from, to)
	case "compliance":
		csvData, err = generator.ComplianceCSV(r.Context(), h.queries)
	default:
		writeError(w, http.StatusBadRequest, "GOLD.REPORTING.VALIDATION", "unknown report type")
		return
	}
	if err != nil {
		h.log.Error("generate csv", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.REPORTING.INTERNAL", "failed to generate csv")
		return
	}

	filename := fmt.Sprintf("report_%s_%s.csv", job.ReportType, job.ID.String()[:8])
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(csvData)
}

// ── middleware ───────────────────────────────────────────────────────────────

func (h *Handlers) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-Admin-Secret")
		if subtle.ConstantTimeCompare([]byte(secret), []byte(h.adminSecret)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid admin secret")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── helpers ─────────────────────────────────────────────────────────────────

// parseStoredDateRange parses date strings stored in job parameters.
func parseStoredDateRange(fromStr, toStr string) (time.Time, time.Time) {
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		from = time.Now().AddDate(0, -1, 0)
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		to = time.Now()
	}
	return from, to.Add(24*time.Hour - time.Second)
}

func parseDateRange(r *http.Request) (time.Time, time.Time) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		from = time.Now().AddDate(0, -1, 0) // default: 1 month ago
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		to = time.Now()
	}
	return from, to.Add(24*time.Hour - time.Second) // end of day
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error":   code,
		"message": message,
	})
}

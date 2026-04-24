// Package http provides the Vault Integration service's HTTP API.
package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ismetaba/gold-token/backend/pkg/httputil"
	"github.com/ismetaba/gold-token/backend/services/vault/internal/domain"
	vaultevents "github.com/ismetaba/gold-token/backend/services/vault/internal/events"
	"github.com/ismetaba/gold-token/backend/services/vault/internal/repo"
)

// Handlers wires together repos and events for the vault HTTP API.
type Handlers struct {
	bars        repo.BarRepo
	movements   repo.MovementRepo
	audits      repo.AuditRepo
	vaults      repo.VaultRepo
	pub         *vaultevents.Publisher
	adminSecret string
	log         *zap.Logger
}

// NewHandlers constructs the handler set.
func NewHandlers(
	bars repo.BarRepo,
	movements repo.MovementRepo,
	audits repo.AuditRepo,
	vaults repo.VaultRepo,
	pub *vaultevents.Publisher,
	adminSecret string,
	log *zap.Logger,
) *Handlers {
	return &Handlers{
		bars: bars, movements: movements, audits: audits, vaults: vaults,
		pub: pub, adminSecret: adminSecret, log: log,
	}
}

// Routes returns the configured chi router.
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

	rl := httputil.NewRateLimiter(120, time.Minute)
	r.Use(rl.Middleware)

	r.Get("/health", h.health)

	r.Route("/vault", func(r chi.Router) {
		r.Use(h.requireAdmin)
		r.Post("/bars/ingest", h.ingestBar)
		r.Get("/bars", h.listBars)
		r.Post("/bars/{serial}/transfer", h.transferBar)
		r.Get("/audits", h.listAudits)
		r.Post("/audits", h.createAudit)
	})

	return r
}

func (h *Handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /vault/bars/ingest
func (h *Handlers) ingestBar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SerialNo       string `json:"serial_no"`
		VaultID        string `json:"vault_id"`
		WeightGramsWei string `json:"weight_grams_wei"`
		Purity9999     int    `json:"purity_9999"`
		RefinerLBMAID  string `json:"refiner_lbma_id"`
		CastDate       string `json:"cast_date"` // YYYY-MM-DD
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.VAULT.VALIDATION", "invalid request body")
		return
	}

	if req.SerialNo == "" {
		writeError(w, http.StatusUnprocessableEntity, "GOLD.VAULT.VALIDATION", "serial_no is required")
		return
	}

	vaultID, err := uuid.Parse(req.VaultID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "GOLD.VAULT.003", "invalid vault_id")
		return
	}

	// Validate vault exists.
	if _, err := h.vaults.ByID(r.Context(), vaultID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "GOLD.VAULT.003", "vault not found")
			return
		}
		h.log.Error("fetch vault", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.VAULT.INTERNAL", "internal error")
		return
	}

	weightWei, ok := new(big.Int).SetString(req.WeightGramsWei, 10)
	if !ok || weightWei.Sign() <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "GOLD.VAULT.VALIDATION", "weight_grams_wei must be a positive integer")
		return
	}

	castDate, err := time.Parse("2006-01-02", req.CastDate)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "GOLD.VAULT.VALIDATION", "cast_date must be YYYY-MM-DD")
		return
	}

	now := time.Now().UTC()
	bar := domain.GoldBar{
		SerialNo:       req.SerialNo,
		VaultID:        vaultID,
		WeightGramsWei: weightWei,
		Purity9999:     req.Purity9999,
		RefinerLBMAID:  req.RefinerLBMAID,
		CastDate:       castDate,
		Status:         domain.BarAvailable,
		IngestedAt:     now,
	}

	if err := h.bars.Ingest(r.Context(), bar); err != nil {
		if errors.Is(err, repo.ErrDuplicateBar) {
			writeError(w, http.StatusConflict, "GOLD.VAULT.002", "bar with this serial already exists")
			return
		}
		h.log.Error("ingest bar", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.VAULT.INTERNAL", "failed to ingest bar")
		return
	}

	// Record movement.
	mvt := domain.BarMovement{
		ID:          uuid.Must(uuid.NewV7()),
		BarSerial:   req.SerialNo,
		ToVaultID:   &vaultID,
		Type:        "ingestion",
		InitiatedBy: "admin",
		Reason:      "initial ingestion",
		MovedAt:     now,
	}
	_ = h.movements.Create(r.Context(), mvt)

	h.pub.Publish(r.Context(), vaultevents.SubjBarIngested, req.SerialNo, map[string]any{
		"serial_no":        req.SerialNo,
		"vault_id":         vaultID.String(),
		"weight_grams_wei": weightWei.String(),
		"purity_9999":      req.Purity9999,
	})

	writeJSON(w, http.StatusCreated, map[string]string{
		"serial_no": req.SerialNo,
		"status":    string(domain.BarAvailable),
	})
}

// GET /vault/bars?vault_id=xxx&status=available&limit=50&offset=0
func (h *Handlers) listBars(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	if limit > 200 {
		limit = 200
	}

	var vaultID *uuid.UUID
	if v := q.Get("vault_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "GOLD.VAULT.VALIDATION", "invalid vault_id")
			return
		}
		vaultID = &id
	}

	var status *string
	if v := q.Get("status"); v != "" {
		status = &v
	}

	bars, err := h.bars.List(r.Context(), vaultID, status, limit, offset)
	if err != nil {
		h.log.Error("list bars", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.VAULT.INTERNAL", "failed to list bars")
		return
	}

	type barResp struct {
		SerialNo        string `json:"serial_no"`
		VaultID         string `json:"vault_id"`
		WeightGramsWei  string `json:"weight_grams_wei"`
		AllocatedSumWei string `json:"allocated_sum_wei"`
		Purity9999      int    `json:"purity_9999"`
		RefinerLBMAID   string `json:"refiner_lbma_id"`
		CastDate        string `json:"cast_date"`
		Status          string `json:"status"`
		IngestedAt      string `json:"ingested_at"`
	}

	out := make([]barResp, 0, len(bars))
	for _, b := range bars {
		out = append(out, barResp{
			SerialNo:        b.SerialNo,
			VaultID:         b.VaultID.String(),
			WeightGramsWei:  b.WeightGramsWei.String(),
			AllocatedSumWei: b.AllocatedSumWei.String(),
			Purity9999:      b.Purity9999,
			RefinerLBMAID:   b.RefinerLBMAID,
			CastDate:        b.CastDate.Format("2006-01-02"),
			Status:          string(b.Status),
			IngestedAt:      b.IngestedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"bars": out, "limit": limit, "offset": offset})
}

// POST /vault/bars/{serial}/transfer
func (h *Handlers) transferBar(w http.ResponseWriter, r *http.Request) {
	serial := chi.URLParam(r, "serial")

	var req struct {
		ToVaultID string `json:"to_vault_id"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.VAULT.VALIDATION", "invalid request body")
		return
	}

	toVaultID, err := uuid.Parse(req.ToVaultID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "GOLD.VAULT.003", "invalid to_vault_id")
		return
	}

	// Validate target vault exists.
	if _, err := h.vaults.ByID(r.Context(), toVaultID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "GOLD.VAULT.003", "target vault not found")
			return
		}
		h.log.Error("fetch target vault", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.VAULT.INTERNAL", "internal error")
		return
	}

	// Get current bar state.
	bar, err := h.bars.BySerial(r.Context(), serial)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "GOLD.VAULT.001", "bar not found")
			return
		}
		h.log.Error("fetch bar", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.VAULT.INTERNAL", "internal error")
		return
	}

	if err := h.bars.UpdateVault(r.Context(), serial, toVaultID); err != nil {
		if errors.Is(err, repo.ErrBarAllocated) {
			writeError(w, http.StatusConflict, "GOLD.VAULT.004", "bar is currently allocated — cannot transfer")
			return
		}
		h.log.Error("transfer bar", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.VAULT.INTERNAL", "transfer failed")
		return
	}

	now := time.Now().UTC()
	fromVault := bar.VaultID
	mvt := domain.BarMovement{
		ID:          uuid.Must(uuid.NewV7()),
		BarSerial:   serial,
		FromVaultID: &fromVault,
		ToVaultID:   &toVaultID,
		Type:        "transfer",
		InitiatedBy: "admin",
		Reason:      req.Reason,
		MovedAt:     now,
	}
	_ = h.movements.Create(r.Context(), mvt)

	h.pub.Publish(r.Context(), vaultevents.SubjBarTransferred, serial, map[string]any{
		"serial_no":     serial,
		"from_vault_id": fromVault.String(),
		"to_vault_id":   toVaultID.String(),
		"reason":        req.Reason,
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"serial_no":   serial,
		"from_vault":  fromVault.String(),
		"to_vault":    toVaultID.String(),
		"status":      "transferred",
	})
}

// GET /vault/audits?vault_id=xxx&limit=50&offset=0
func (h *Handlers) listAudits(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	if limit > 200 {
		limit = 200
	}

	var vaultID *uuid.UUID
	if v := r.URL.Query().Get("vault_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "GOLD.VAULT.VALIDATION", "invalid vault_id")
			return
		}
		vaultID = &id
	}

	records, err := h.audits.List(r.Context(), vaultID, limit, offset)
	if err != nil {
		h.log.Error("list audits", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.VAULT.INTERNAL", "failed to list audits")
		return
	}

	type auditResp struct {
		ID             string          `json:"id"`
		VaultID        string          `json:"vault_id"`
		Auditor        string          `json:"auditor"`
		AuditType      string          `json:"audit_type"`
		BarCount       int             `json:"bar_count"`
		TotalWeightWei string          `json:"total_weight_grams_wei"`
		Discrepancies  json.RawMessage `json:"discrepancies"`
		Status         string          `json:"status"`
		AuditedAt      string          `json:"audited_at"`
	}

	out := make([]auditResp, 0, len(records))
	for _, a := range records {
		out = append(out, auditResp{
			ID:             a.ID.String(),
			VaultID:        a.VaultID.String(),
			Auditor:        a.Auditor,
			AuditType:      a.AuditType,
			BarCount:       a.BarCount,
			TotalWeightWei: a.TotalWeightWei.String(),
			Discrepancies:  a.Discrepancies,
			Status:         a.Status,
			AuditedAt:      a.AuditedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"audits": out, "limit": limit, "offset": offset})
}

// POST /vault/audits
func (h *Handlers) createAudit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VaultID        string          `json:"vault_id"`
		Auditor        string          `json:"auditor"`
		AuditType      string          `json:"audit_type"`
		BarCount       int             `json:"bar_count"`
		TotalWeightWei string          `json:"total_weight_grams_wei"`
		Discrepancies  json.RawMessage `json:"discrepancies"`
		Status         string          `json:"status"`
		AuditedAt      string          `json:"audited_at"` // RFC3339
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "GOLD.VAULT.VALIDATION", "invalid request body")
		return
	}

	vaultID, err := uuid.Parse(req.VaultID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "GOLD.VAULT.003", "invalid vault_id")
		return
	}

	weightWei, ok := new(big.Int).SetString(req.TotalWeightWei, 10)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "GOLD.VAULT.VALIDATION", "total_weight_grams_wei must be a valid integer")
		return
	}

	auditedAt, err := time.Parse(time.RFC3339, req.AuditedAt)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "GOLD.VAULT.VALIDATION", "audited_at must be RFC3339")
		return
	}

	if req.Status != "passed" && req.Status != "discrepancy" {
		writeError(w, http.StatusUnprocessableEntity, "GOLD.VAULT.VALIDATION", "status must be 'passed' or 'discrepancy'")
		return
	}

	now := time.Now().UTC()
	record := domain.AuditRecord{
		ID:             uuid.Must(uuid.NewV7()),
		VaultID:        vaultID,
		Auditor:        req.Auditor,
		AuditType:      req.AuditType,
		BarCount:       req.BarCount,
		TotalWeightWei: weightWei,
		Discrepancies:  req.Discrepancies,
		Status:         req.Status,
		AuditedAt:      auditedAt,
		RecordedAt:     now,
	}

	if err := h.audits.Create(r.Context(), record); err != nil {
		h.log.Error("create audit record", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "GOLD.VAULT.INTERNAL", "failed to create audit record")
		return
	}

	h.pub.Publish(r.Context(), vaultevents.SubjAuditCompleted, vaultID.String(), map[string]any{
		"audit_id":  record.ID.String(),
		"vault_id":  vaultID.String(),
		"status":    req.Status,
		"bar_count": req.BarCount,
	})

	writeJSON(w, http.StatusCreated, map[string]string{
		"audit_id": record.ID.String(),
		"status":   req.Status,
	})
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

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
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

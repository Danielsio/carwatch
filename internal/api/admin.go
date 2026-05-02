package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"
)

type adminListingResponse struct {
	ChatID       int64    `json:"chat_id"`
	Token        string   `json:"token"`
	SearchName   string   `json:"search_name,omitempty"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	Year         int      `json:"year"`
	Price        int      `json:"price"`
	Km           int      `json:"km"`
	Hand         int      `json:"hand"`
	City         string   `json:"city"`
	PageLink     string   `json:"page_link"`
	ImageURL     string   `json:"image_url,omitempty"`
	FitnessScore *float64 `json:"fitness_score,omitempty"`
	FirstSeenAt  string   `json:"first_seen_at"`
}

type adminStatsResponse struct {
	DB      dbStats      `json:"db"`
	Tables  map[string]int64 `json:"tables"`
	Runtime runtimeStats `json:"runtime"`
}

type dbStats struct {
	FileSizeBytes int64  `json:"file_size_bytes"`
	FileSizeHuman string `json:"file_size_human"`
}

type runtimeStats struct {
	Goroutines int    `json:"goroutines"`
	MemAllocMB float64 `json:"mem_alloc_mb"`
	MemSysMB   float64 `json:"mem_sys_mb"`
	Uptime     string `json:"uptime"`
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chatID := chatIDFromContext(r.Context())
		email := emailFromContext(r.Context())

		isAdmin := (s.cfg.AdminChatID != 0 && chatID == s.cfg.AdminChatID) ||
			(s.cfg.AdminEmail != "" && email != "" && email == s.cfg.AdminEmail)

		if !isAdmin {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next(w, r)
	}
}

func (s *Server) adminStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fileSize, err := s.admin.DBFileSize()
	if err != nil {
		s.logger.Error("admin: db file size", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get db file size")
		return
	}

	tables, err := s.admin.TableSizes(ctx)
	if err != nil {
		s.logger.Error("admin: table sizes", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get table sizes")
		return
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	writeJSON(w, http.StatusOK, adminStatsResponse{
		DB: dbStats{
			FileSizeBytes: fileSize,
			FileSizeHuman: humanBytes(fileSize),
		},
		Tables: tables,
		Runtime: runtimeStats{
			Goroutines: runtime.NumGoroutine(),
			MemAllocMB: float64(mem.Alloc) / 1024 / 1024,
			MemSysMB:   float64(mem.Sys) / 1024 / 1024,
			Uptime:     time.Since(s.startTime).Truncate(time.Second).String(),
		},
	})
}

func humanBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func (s *Server) adminPurgeTable(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Table string `json:"table"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Table == "" {
		writeError(w, http.StatusBadRequest, "table is required")
		return
	}

	deleted, err := s.admin.PurgeTable(r.Context(), body.Table)
	if err != nil {
		s.logger.Error("admin: purge table", "table", body.Table, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("admin: purged table", "table", body.Table, "deleted", deleted)
	writeJSON(w, http.StatusOK, map[string]any{"table": body.Table, "deleted": deleted})
}

func (s *Server) adminListListings(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	if limit > 100 {
		limit = 100
	}
	offset := parseIntParam(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}

	items, total, err := s.admin.AdminListListings(r.Context(), limit, offset)
	if err != nil {
		s.logger.Error("admin: list listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list listings")
		return
	}

	resp := make([]adminListingResponse, 0, len(items))
	for _, l := range items {
		resp = append(resp, adminListingResponse{
			ChatID:       l.ChatID,
			Token:        l.Token,
			SearchName:   l.SearchName,
			Manufacturer: l.Manufacturer,
			Model:        l.Model,
			Year:         l.Year,
			Price:        l.Price,
			Km:           l.Km,
			Hand:         l.Hand,
			City:         l.City,
			PageLink:     l.PageLink,
			ImageURL:     l.ImageURL,
			FitnessScore: l.FitnessScore,
			FirstSeenAt:  l.FirstSeenAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  resp,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) adminDeleteListing(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	var body struct {
		ChatID int64 `json:"chat_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ChatID == 0 {
		writeError(w, http.StatusBadRequest, "chat_id is required in body")
		return
	}

	if err := s.admin.AdminDeleteListing(r.Context(), token, body.ChatID); err != nil {
		s.logger.Error("admin: delete listing", "token", token, "chat_id", body.ChatID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete listing")
		return
	}
	s.logger.Info("admin: deleted listing", "token", token, "chat_id", body.ChatID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminVacuum(w http.ResponseWriter, r *http.Request) {
	if err := s.admin.VacuumDB(r.Context()); err != nil {
		s.logger.Error("admin: vacuum", "error", err)
		writeError(w, http.StatusInternalServerError, "vacuum failed")
		return
	}

	fileSize, err := s.admin.DBFileSize()
	if err != nil {
		s.logger.Warn("admin: vacuum succeeded but size read failed", "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	s.logger.Info("admin: vacuum complete", "size_after", humanBytes(fileSize))
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"size_after": humanBytes(fileSize),
		"size_bytes": fileSize,
	})
}

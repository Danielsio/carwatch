package api

import (
	"fmt"
	"net/http"
	"runtime"
	"time"
)

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

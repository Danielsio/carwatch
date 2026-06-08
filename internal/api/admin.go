package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dsionov/carwatch/internal/logstream"
	"github.com/dsionov/carwatch/internal/storage"
)

type adminListingResponse struct {
	ChatID       int64    `json:"chat_id"`
	Token        string   `json:"token"`
	SearchID     int64    `json:"search_id"`
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
	IsCommercial *bool    `json:"is_commercial,omitempty"`
	SubModel     string   `json:"sub_model,omitempty"`
	EngineVolume float64  `json:"engine_volume,omitempty"`
	HorsePower   int      `json:"horse_power,omitempty"`
	EngineType   string   `json:"engine_type,omitempty"`
	GearBox      string   `json:"gear_box,omitempty"`
	Description  string   `json:"description,omitempty"`
}

type adminStatsResponse struct {
	DB      dbStats              `json:"db"`
	Tables  map[string]int64     `json:"tables"`
	Runtime runtimeStats         `json:"runtime"`
	HTTP    httpStats            `json:"http"`
	Pool    *storage.DBPoolStats `json:"pool,omitempty"`
	Vitals  []vitalsSummary      `json:"vitals,omitempty"`
}

type dbStats struct {
	FileSizeBytes int64  `json:"file_size_bytes"`
	FileSizeHuman string `json:"file_size_human"`
}

type runtimeStats struct {
	Goroutines int     `json:"goroutines"`
	MemAllocMB float64 `json:"mem_alloc_mb"`
	MemSysMB   float64 `json:"mem_sys_mb"`
	Uptime     string  `json:"uptime"`
}

type httpStats struct {
	RequestsTotal uint64  `json:"requests_total"`
	Status2xx     uint64  `json:"status_2xx"`
	Status4xx     uint64  `json:"status_4xx"`
	Status5xx     uint64  `json:"status_5xx"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

func (s *Server) isAdmin(chatID int64, email string) bool {
	return (chatID > 0 && s.cfg.AdminChatID != 0 && chatID == s.cfg.AdminChatID) ||
		(s.cfg.AdminEmail != "" && email != "" && strings.EqualFold(email, s.cfg.AdminEmail))
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	chatID, _ := chatIDFromContext(r.Context())
	email := emailFromContext(r.Context())

	writeJSON(w, http.StatusOK, map[string]any{
		"email":    email,
		"is_admin": s.isAdmin(chatID, email),
	})
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chatID, _ := chatIDFromContext(r.Context())
		email := emailFromContext(r.Context())

		if !s.isAdmin(chatID, email) {
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

	total, n2xx, n4xx, n5xx, durSum := s.httpMetricsSnapshot()
	var avgMs float64
	if total > 0 {
		avgMs = float64(durSum) / float64(total)
	}

	pool := s.admin.DBPoolStats()

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
		HTTP: httpStats{
			RequestsTotal: total,
			Status2xx:     n2xx,
			Status4xx:     n4xx,
			Status5xx:     n5xx,
			AvgDurationMs: avgMs,
		},
		Pool:   pool,
		Vitals: s.vitals.summarize(),
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
		if errors.Is(err, storage.ErrNotPurgeable) {
			writeError(w, http.StatusBadRequest, "purge operation failed")
		} else {
			writeError(w, http.StatusInternalServerError, "purge operation failed")
		}
		return
	}
	s.logger.Info("admin: purged table", "table", body.Table, "deleted", deleted)
	writeJSON(w, http.StatusOK, map[string]any{"table": body.Table, "deleted": deleted})
}

func (s *Server) adminListListings(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseIntParam(w, r, "limit", 50)
	if !ok {
		return
	}
	if limit > 100 {
		limit = 100
	}
	offset, ok := parseIntParam(w, r, "offset", 0)
	if !ok {
		return
	}
	if offset < 0 {
		offset = 0
	}
	searchIDVal, ok := parseIntParam(w, r, "search_id", 0)
	if !ok {
		return
	}
	searchID := int64(searchIDVal)

	items, total, err := s.admin.AdminListListings(r.Context(), limit, offset, searchID)
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
			SearchID:     l.SearchID,
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
			IsCommercial: l.IsCommercial,
			SubModel:     l.SubModel,
			EngineVolume: l.EngineVolume,
			HorsePower:   l.HorsePower,
			EngineType:   l.EngineType,
			GearBox:      l.GearBox,
			Description:  l.Description,
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

func (s *Server) adminListSearches(w http.ResponseWriter, r *http.Request) {
	searches, err := s.admin.AdminListSearches(r.Context())
	if err != nil {
		s.logger.Error("admin: list searches", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list searches")
		return
	}

	type searchResp struct {
		ID           int64  `json:"id"`
		ChatID       int64  `json:"chat_id"`
		Name         string `json:"name"`
		Source       string `json:"source"`
		Manufacturer int    `json:"manufacturer"`
		Model        int    `json:"model"`
		YearMin      int    `json:"year_min"`
		YearMax      int    `json:"year_max"`
		PriceMin     int    `json:"price_min"`
		PriceMax     int    `json:"price_max"`
		MaxKm        int    `json:"max_km"`
		MaxHand      int    `json:"max_hand"`
		EngineMinCC  int    `json:"engine_min_cc"`
		Keywords     string `json:"keywords,omitempty"`
		ExcludeKeys  string `json:"exclude_keys,omitempty"`
		SellerFilter string `json:"seller_filter,omitempty"`
		Active       bool   `json:"active"`
		CreatedAt    string `json:"created_at"`
	}

	resp := make([]searchResp, 0, len(searches))
	for _, s := range searches {
		resp = append(resp, searchResp{
			ID:           s.ID,
			ChatID:       s.ChatID,
			Name:         s.Name,
			Source:       s.Source,
			Manufacturer: s.Manufacturer,
			Model:        s.Model,
			YearMin:      s.YearMin,
			YearMax:      s.YearMax,
			PriceMin:     s.PriceMin,
			PriceMax:     s.PriceMax,
			MaxKm:        s.MaxKm,
			MaxHand:      s.MaxHand,
			EngineMinCC:  s.EngineMinCC,
			Keywords:     s.Keywords,
			ExcludeKeys:  s.ExcludeKeys,
			SellerFilter: storage.NormalizeSellerFilter(s.SellerFilter),
			Active:       s.Active,
			CreatedAt:    s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": resp, "total": len(resp)})
}

func (s *Server) adminDeleteSearch(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "valid search id is required")
		return
	}

	if err := s.admin.AdminDeleteSearch(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "search not found")
			return
		}
		s.logger.Error("admin: delete search", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete search")
		return
	}
	s.logger.Info("admin: deleted search", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.admin.AdminListUsers(r.Context())
	if err != nil {
		s.logger.Error("admin: list users", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	type userResp struct {
		ChatID    int64  `json:"chat_id"`
		Username  string `json:"username"`
		Channel   string `json:"channel"`
		ChannelID string `json:"channel_id"`
		Active    bool   `json:"active"`
		Tier      string `json:"tier"`
		Language  string `json:"language"`
		CreatedAt string `json:"created_at"`
	}

	resp := make([]userResp, 0, len(users))
	for _, u := range users {
		resp = append(resp, userResp{
			ChatID:    u.ChatID,
			Username:  u.Username,
			Channel:   u.Channel,
			ChannelID: u.ChannelID,
			Active:    u.Active,
			Tier:      u.Tier,
			Language:  u.Language,
			CreatedAt: u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": resp, "total": len(resp)})
}

func (s *Server) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("chatID")
	chatID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || chatID <= 0 {
		writeError(w, http.StatusBadRequest, "valid chat_id is required")
		return
	}

	if err := s.admin.AdminDeleteUser(r.Context(), chatID); err != nil {
		s.logger.Error("admin: delete user", "chat_id", chatID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	s.logger.Info("admin: deleted user", "chat_id", chatID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminSetUserActive(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("chatID")
	chatID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || chatID <= 0 {
		writeError(w, http.StatusBadRequest, "valid chat_id is required")
		return
	}

	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.users.SetUserActive(r.Context(), chatID, body.Active); err != nil {
		s.logger.Error("admin: set user active", "chat_id", chatID, "active", body.Active, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	s.logger.Info("admin: updated user active status", "chat_id", chatID, "active", body.Active)
	writeJSON(w, http.StatusOK, map[string]any{"chat_id": chatID, "active": body.Active})
}

func (s *Server) adminSyncUserStatus(w http.ResponseWriter, r *http.Request) {
	activated, deactivated, err := s.admin.SyncUserActiveStatus(r.Context())
	if err != nil {
		s.logger.Error("admin: sync user status", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to sync user status")
		return
	}
	s.logger.Info("admin: synced user active status", "activated", activated, "deactivated", deactivated)
	writeJSON(w, http.StatusOK, map[string]any{
		"activated":   activated,
		"deactivated": deactivated,
	})
}

func (s *Server) adminVacuum(w http.ResponseWriter, r *http.Request) {
	if !s.vacuumMu.TryLock() {
		writeError(w, http.StatusConflict, "vacuum already in progress")
		return
	}
	defer s.vacuumMu.Unlock()

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

func (s *Server) adminListPriceHistory(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseIntParam(w, r, "limit", 50)
	if !ok {
		return
	}
	if limit > 100 {
		limit = 100
	}
	offset, ok := parseIntParam(w, r, "offset", 0)
	if !ok {
		return
	}
	if offset < 0 {
		offset = 0
	}
	token := r.URL.Query().Get("token")

	items, total, err := s.admin.AdminListPriceHistory(r.Context(), limit, offset, token)
	if err != nil {
		s.logger.Error("admin: list price history", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list price history")
		return
	}

	type priceResp struct {
		Token        string `json:"token"`
		Price        int    `json:"price"`
		ObservedAt   string `json:"observed_at"`
		Manufacturer string `json:"manufacturer,omitempty"`
		Model        string `json:"model,omitempty"`
		Year         int    `json:"year,omitempty"`
	}

	resp := make([]priceResp, 0, len(items))
	for _, p := range items {
		resp = append(resp, priceResp{
			Token:        p.Token,
			Price:        p.Price,
			ObservedAt:   p.ObservedAt.UTC().Format("2006-01-02T15:04:05Z"),
			Manufacturer: p.Manufacturer,
			Model:        p.Model,
			Year:         p.Year,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": resp, "total": total, "limit": limit, "offset": offset})
}

func (s *Server) adminListSeenListings(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseIntParam(w, r, "limit", 50)
	if !ok {
		return
	}
	if limit > 100 {
		limit = 100
	}
	offset, ok := parseIntParam(w, r, "offset", 0)
	if !ok {
		return
	}
	if offset < 0 {
		offset = 0
	}
	searchIDVal, ok := parseIntParam(w, r, "search_id", 0)
	if !ok {
		return
	}

	items, total, err := s.admin.AdminListSeenListings(r.Context(), limit, offset, int64(searchIDVal))
	if err != nil {
		s.logger.Error("admin: list seen listings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list seen listings")
		return
	}

	type seenResp struct {
		Token       string `json:"token"`
		ChatID      int64  `json:"chat_id"`
		SearchID    int64  `json:"search_id"`
		FirstSeenAt string `json:"first_seen_at"`
	}

	resp := make([]seenResp, 0, len(items))
	for _, sr := range items {
		resp = append(resp, seenResp{
			Token:       sr.Token,
			ChatID:      sr.ChatID,
			SearchID:    sr.SearchID,
			FirstSeenAt: sr.FirstSeenAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": resp, "total": total, "limit": limit, "offset": offset})
}

func (s *Server) adminActivity(w http.ResponseWriter, r *http.Request) {
	days, ok := parseIntParam(w, r, "days", 30)
	if !ok {
		return
	}
	if days < 1 {
		days = 1
	}
	if days > 365 {
		days = 365
	}

	stats, err := s.admin.AdminActivityStats(r.Context(), days)
	if err != nil {
		s.logger.Error("admin: activity stats", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get activity stats")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": stats})
}

func (s *Server) adminCycles(w http.ResponseWriter, r *http.Request) {
	limit, ok := parseIntParam(w, r, "limit", 50)
	if !ok {
		return
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	entries, err := s.cycleLog.ListCycleLogs(r.Context(), limit)
	if err != nil {
		s.logger.Error("admin: list cycle logs", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list cycle logs")
		return
	}

	type cycleResponse struct {
		ID              int64  `json:"id"`
		StartedAt       string `json:"started_at"`
		DurationMs      int    `json:"duration_ms"`
		Searches        int    `json:"searches"`
		ListingsFetched int    `json:"listings_fetched"`
		ListingsMatched int    `json:"listings_matched"`
		Notifications   int    `json:"notifications"`
		ErrorMessage    string `json:"error_message,omitempty"`
		Status          string `json:"status"`
	}

	resp := make([]cycleResponse, 0, len(entries))
	for _, e := range entries {
		resp = append(resp, cycleResponse{
			ID:              e.ID,
			StartedAt:       e.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
			DurationMs:      e.DurationMs,
			Searches:        e.Searches,
			ListingsFetched: e.ListingsFetched,
			ListingsMatched: e.ListingsMatched,
			Notifications:   e.Notifications,
			ErrorMessage:    e.ErrorMessage,
			Status:          e.Status,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": resp})
}

func (s *Server) adminLogs(w http.ResponseWriter, r *http.Request) {
	n, ok := parseIntParam(w, r, "limit", 2000)
	if !ok {
		return
	}
	if n > 10000 {
		n = 10000
	}
	entries := s.logHub.Recent(n)
	if entries == nil {
		entries = []logstream.LogEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": entries})
}

func (s *Server) adminLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	rc := http.NewResponseController(w)
	ch, unsub := s.logHub.Subscribe()
	defer unsub()

	ctx := r.Context()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if err := rc.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
				return
			}
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case entry := <-ch:
			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			if err := rc.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

var validLogLevels = map[string]slog.Level{
	"DEBUG": slog.LevelDebug,
	"INFO":  slog.LevelInfo,
	"WARN":  slog.LevelWarn,
	"ERROR": slog.LevelError,
}

func (s *Server) adminGetLogLevel(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"level": s.logLevel.Level().String()})
}

func (s *Server) adminSetLogLevel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Level string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	body.Level = strings.ToUpper(strings.TrimSpace(body.Level))
	lvl, ok := validLogLevels[body.Level]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid level; must be DEBUG, INFO, WARN, or ERROR")
		return
	}
	s.logLevel.Set(lvl)
	s.logger.Info("log level changed", "level", body.Level)
	writeJSON(w, http.StatusOK, map[string]string{"level": s.logLevel.Level().String()})
}

func (s *Server) adminEnrichmentStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	unenriched, err := s.listings.CountUnenrichedTokens(ctx)
	if err != nil {
		s.logger.Error("admin: count unenriched tokens", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count unenriched tokens")
		return
	}
	resp := map[string]any{
		"unenriched_count": unenriched, // actionable now
	}

	type enrichmentDiagnosticsCounter interface {
		CountUnenrichedCooldownTokens(context.Context) (int64, error)
		CountUnenrichedExhaustedTokens(context.Context) (int64, error)
	}
	if diagStore, ok := s.listings.(enrichmentDiagnosticsCounter); ok {
		if cooldown, diagErr := diagStore.CountUnenrichedCooldownTokens(ctx); diagErr != nil {
			s.logger.Warn("admin: count unenriched cooldown tokens", "error", diagErr)
		} else {
			resp["cooldown_count"] = cooldown
		}
		if exhausted, diagErr := diagStore.CountUnenrichedExhaustedTokens(ctx); diagErr != nil {
			s.logger.Warn("admin: count unenriched exhausted tokens", "error", diagErr)
		} else {
			resp["exhausted_count"] = exhausted
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

type schedulerStatusResponse struct {
	LastCycleAt         *string `json:"last_cycle_at"`
	LastCycleDurationMs int     `json:"last_cycle_duration_ms"`
	LastCycleStatus     string  `json:"last_cycle_status"`
	NextCycleAt         *string `json:"next_cycle_at"`
	PollingIntervalSec  int     `json:"polling_interval_seconds"`
}

func (s *Server) schedulerStatus(w http.ResponseWriter, r *http.Request) {
	log := s.handlerLogger(r, "op", "scheduler_status")

	entries, err := s.cycleLog.ListCycleLogs(r.Context(), 1)
	if err != nil {
		log.Error("failed to list cycle logs", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get cycle log")
		return
	}

	resp := schedulerStatusResponse{
		PollingIntervalSec: int(s.pollingInterval.Seconds()),
	}

	if len(entries) > 0 {
		last := entries[0]
		ts := last.StartedAt.UTC().Format(time.RFC3339)
		resp.LastCycleAt = &ts
		resp.LastCycleDurationMs = last.DurationMs
		resp.LastCycleStatus = last.Status

		next := last.StartedAt.Add(time.Duration(last.DurationMs)*time.Millisecond + s.pollingInterval)
		nextStr := next.UTC().Format(time.RFC3339)
		resp.NextCycleAt = &nextStr
	}

	writeJSON(w, http.StatusOK, resp)
}

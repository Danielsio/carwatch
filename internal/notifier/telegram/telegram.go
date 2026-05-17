package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/locale"
	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
)

const (
	maxMessageLen = 4096
	maxCaptionLen = 1024

	// telegramListingSeparatorLine matches notifier.FormatBatch listing dividers so
	// long batch messages split on whole Markdown blocks instead of arbitrary lines.
	telegramListingSeparatorLine = "━━━━━━━━━━━━━━━━━━━━"

	// defaultSendDelay is the minimum pause between consecutive Telegram
	// API calls to avoid hitting rate limits.
	defaultSendDelay = 50 * time.Millisecond

	// telegramMaxRetries is the maximum number of extra attempts after a
	// rate-limited (HTTP 429) sendMessage/sendPhoto failure.
	telegramMaxRetries = 3

	// telegramRetryBaseDelay is the initial backoff for exponential delay
	// between rate-limit retries; grows as base * 2^n per attempt.
	telegramRetryBaseDelay = 1 * time.Second

	// retryAfterDefault is the fallback sleep duration when the API returns
	// a 429 Too Many Requests error without a parseable retry-after value.
	retryAfterDefault = 5 * time.Second
)

type Notifier struct {
	bot       *tgbot.Bot
	logger    *slog.Logger
	sendDelay time.Duration
}

func New(token string, logger *slog.Logger, opts ...tgbot.Option) (*Notifier, error) {
	defaults := []tgbot.Option{
		tgbot.WithDefaultHandler(func(_ context.Context, _ *tgbot.Bot, _ *tgmodels.Update) {}),
	}
	allOpts := append(defaults, opts...)
	b, err := tgbot.New(token, allOpts...)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}
	return &Notifier{bot: b, logger: logger, sendDelay: defaultSendDelay}, nil
}

func (n *Notifier) Bot() *tgbot.Bot {
	return n.bot
}

func (n *Notifier) Connect(ctx context.Context) error {
	me, err := n.bot.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("telegram getMe: %w", err)
	}
	n.logger.Info("telegram bot connected", "username", me.Username)
	return nil
}

func (n *Notifier) Notify(ctx context.Context, chatID string, listings []model.Listing, lang locale.Lang) error {
	if len(listings) == 1 && listings[0].ImageURL != "" {
		return n.sendListingWithPhoto(ctx, chatID, listings[0], lang)
	}
	msg := notifier.FormatBatch(listings, lang)
	return n.sendMessageMarkdown(ctx, chatID, msg)
}

func (n *Notifier) NotifyRaw(ctx context.Context, chatID string, message string) error {
	n.logger.Debug("notifyRaw",
		"chat_id", chatID,
		"msg_len", len(message),
		"msg_preview", truncate(message, 100),
	)
	return n.sendMessageMarkdown(ctx, chatID, message)
}

func (n *Notifier) Disconnect() error {
	return nil
}

func (n *Notifier) sendListingWithPhoto(ctx context.Context, chatID string, listing model.Listing, lang locale.Lang) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID %q: %w", chatID, err)
	}

	caption := notifier.FormatListing(listing, lang)

	if notifier.IsMalformedMessage(caption) {
		n.logger.Error("blocked malformed photo caption",
			"chat_id", chatID,
			"caption_len", len(caption),
			"caption_preview", truncate(caption, 200),
		)
		return fmt.Errorf("blocked malformed photo caption (%d chars): %q", len(caption), truncate(caption, 100))
	}

	captionForPhoto := caption
	if len([]rune(captionForPhoto)) > maxCaptionLen {
		captionForPhoto = truncateTelegramCaption(captionForPhoto, maxCaptionLen)
	}

	err = n.sendPhotoWithRetry(ctx, chatID, &tgbot.SendPhotoParams{
		ChatID:    id,
		Photo:     &tgmodels.InputFileString{Data: listing.ImageURL},
		Caption:   captionForPhoto,
		ParseMode: tgmodels.ParseModeMarkdownV1,
	})
	if err != nil {
		n.logger.Warn("sendPhoto failed, falling back to text", "chat_id", chatID, "error", err)
		return n.sendMessageMarkdown(ctx, chatID, caption)
	}

	n.logger.Info("sent telegram photo", "chat_id", chatID)
	return nil
}

func (n *Notifier) sendMessageMarkdown(ctx context.Context, chatID string, text string) error {
	return n.sendMessageWithParseMode(ctx, chatID, text, tgmodels.ParseModeMarkdownV1)
}

func (n *Notifier) sendMessageWithParseMode(ctx context.Context, chatID string, text string, parseMode tgmodels.ParseMode) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID %q: %w", chatID, err)
	}

	if notifier.IsMalformedMessage(text) {
		n.logger.Error("blocked malformed message",
			"chat_id", chatID,
			"text_len", len(text),
			"text_preview", truncate(text, 200),
		)
		return fmt.Errorf("blocked malformed message (%d chars): %q", len(text), truncate(text, 100))
	}

	chunks := splitMessage(text, maxMessageLen)
	for i, chunk := range chunks {
		if i > 0 {
			time.Sleep(n.sendDelay)
		}
		params := &tgbot.SendMessageParams{
			ChatID: id,
			Text:   chunk,
		}
		if parseMode != "" {
			params.ParseMode = parseMode
		}
		err = n.sendMessageWithRetry(ctx, chatID, params)
		if err != nil {
			return err
		}
	}

	n.logger.Info("sent telegram message", "chat_id", chatID, "chunks", len(chunks))
	return nil
}

func (n *Notifier) sendMessageWithRetry(ctx context.Context, chatID string, params *tgbot.SendMessageParams) error {
	var err error
	for attempt := 0; ; attempt++ {
		_, err = n.bot.SendMessage(ctx, params)
		if err == nil {
			return nil
		}
		if recErr := recipientBlockedError(err); recErr != nil {
			return recErr
		}
		if isMarkdownParseError(err) && params.ParseMode != "" {
			n.logger.Warn("markdown parse failed, retrying as plain text",
				"chat_id", chatID, "error", err)
			params.ParseMode = ""
			continue
		}
		if isTransientError(err) && attempt < telegramMaxRetries {
			delay := telegramRetryBaseDelay * time.Duration(1<<uint(attempt))
			n.logger.Warn("transient telegram error, retrying",
				"chat_id", chatID, "attempt", attempt+1, "wait", delay.String(), "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			continue
		}
		if !isRateLimited(err) || attempt >= telegramMaxRetries {
			return fmt.Errorf("telegram sendMessage: %w", err)
		}
		delay := telegramRetryBaseDelay * time.Duration(1<<uint(attempt))
		if w := parseRetryAfter(err); w > delay {
			delay = w
		}
		n.logger.Warn("rate limited by telegram, retrying",
			"chat_id", chatID, "attempt", attempt+1, "wait", delay.String())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (n *Notifier) sendPhotoWithRetry(ctx context.Context, chatID string, params *tgbot.SendPhotoParams) error {
	var err error
	for attempt := 0; ; attempt++ {
		_, err = n.bot.SendPhoto(ctx, params)
		if err == nil {
			return nil
		}
		if recErr := recipientBlockedError(err); recErr != nil {
			return recErr
		}
		if !isRateLimited(err) || attempt >= telegramMaxRetries {
			return err
		}
		delay := telegramRetryBaseDelay * time.Duration(1<<uint(attempt))
		if w := parseRetryAfter(err); w > delay {
			delay = w
		}
		n.logger.Warn("rate limited by telegram on sendPhoto, retrying",
			"chat_id", chatID, "attempt", attempt+1, "wait", delay.String())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func recipientBlockedError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "bot was blocked by the user") ||
		strings.Contains(errMsg, "user is deactivated") ||
		strings.Contains(errMsg, "chat not found") {
		return fmt.Errorf("%w: %w", notifier.ErrRecipientBlocked, err)
	}
	return nil
}

func splitMessage(text string, limit int) []string {
	r := []rune(text)
	if len(r) <= limit {
		return []string{text}
	}

	sep := "\n" + telegramListingSeparatorLine + "\n"
	parts := strings.Split(text, sep)
	if len(parts) > 1 {
		return packListingChunks(parts, sep, limit)
	}
	return splitMessageByNewlines(text, limit)
}

// packListingChunks merges batch header and per-listing sections without exceeding
// limit runes per chunk, preserving listing separator lines as boundaries.
func packListingChunks(parts []string, sep string, limit int) []string {
	type atom struct {
		text           string
		needsSepBefore bool
	}
	var atoms []atom
	for i, p := range parts {
		needsSep := i > 0
		for j, sc := range splitMessageByNewlines(p, limit) {
			atoms = append(atoms, atom{text: sc, needsSepBefore: needsSep && j == 0})
		}
	}

	var chunks []string
	var b strings.Builder
	curRunes := 0
	for _, a := range atoms {
		prefix := ""
		if a.needsSepBefore {
			prefix = sep
		}
		next := prefix + a.text
		nr := len([]rune(next))
		if b.Len() == 0 {
			if nr > limit {
				chunks = append(chunks, splitMessageByNewlines(next, limit)...)
				continue
			}
			b.WriteString(next)
			curRunes = nr
			continue
		}
		if curRunes+nr <= limit {
			b.WriteString(next)
			curRunes += nr
			continue
		}
		chunks = append(chunks, b.String())
		b.Reset()
		b.WriteString(next)
		curRunes = nr
	}
	if b.Len() > 0 {
		chunks = append(chunks, b.String())
	}
	return chunks
}

func splitMessageByNewlines(text string, limit int) []string {
	r := []rune(text)
	if len(r) <= limit {
		return []string{text}
	}
	var chunks []string
	for len(r) > 0 {
		if len(r) <= limit {
			chunks = append(chunks, string(r))
			break
		}
		cut := limit
		if idx := lastRuneNewlineBefore(r, limit); idx > 0 {
			cut = idx + 1
		}
		chunks = append(chunks, string(r[:cut]))
		r = r[cut:]
	}
	return chunks
}

// truncateTelegramCaption fits caption within maxRunes (Telegram limit), preferring
// a cut at the last newline before the limit and appending an ellipsis.
func truncateTelegramCaption(caption string, maxRunes int) string {
	r := []rune(caption)
	if len(r) <= maxRunes {
		return caption
	}
	const ellipsis = "…"
	el := []rune(ellipsis)
	maxBody := maxRunes - len(el)
	if maxBody <= 0 {
		return string(el[:maxRunes])
	}
	for i := maxBody - 1; i >= 0; i-- {
		if r[i] == '\n' {
			return string(r[:i]) + ellipsis
		}
	}
	return string(r[:maxBody]) + ellipsis
}

func lastRuneNewlineBefore(s []rune, pos int) int {
	if pos > len(s) {
		pos = len(s)
	}
	for i := pos - 1; i >= 0; i-- {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}

func isMarkdownParseError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "can't parse") ||
		(strings.Contains(msg, "entities") && strings.Contains(msg, "parse"))
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "gateway")
}

func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "retry_after")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// parseRetryAfter attempts to extract the retry-after duration from a
// Telegram error message. The go-telegram/bot library embeds the value
// in the error string (e.g. "...retry after 5"). If parsing fails the
// default of 5 seconds is returned.
func parseRetryAfter(err error) time.Duration {
	if err == nil {
		return retryAfterDefault
	}
	msg := strings.ToLower(err.Error())
	for _, prefix := range []string{"retry after ", "retry_after\":"} {
		idx := strings.Index(msg, prefix)
		if idx < 0 {
			continue
		}
		numStr := msg[idx+len(prefix):]
		numStr = strings.TrimLeft(numStr, " ")
		end := 0
		for end < len(numStr) && numStr[end] >= '0' && numStr[end] <= '9' {
			end++
		}
		if end > 0 {
			secs, convErr := strconv.Atoi(numStr[:end])
			if convErr == nil && secs > 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return retryAfterDefault
}

package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
)

type Notifier struct {
	bot    *tgbot.Bot
	logger *slog.Logger
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
	return &Notifier{bot: b, logger: logger}, nil
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

func (n *Notifier) Notify(ctx context.Context, chatID string, listings []model.Listing) error {
	if len(listings) == 1 && listings[0].ImageURL != "" {
		return n.sendListingWithPhoto(ctx, chatID, listings[0])
	}
	msg := notifier.FormatBatch(listings)
	return n.sendMessage(ctx, chatID, msg)
}

func (n *Notifier) NotifyRaw(ctx context.Context, chatID string, message string) error {
	return n.sendMessage(ctx, chatID, message)
}

func (n *Notifier) Disconnect() error {
	return nil
}

const (
	maxMessageLen = 4096
	maxCaptionLen = 1024
)

func (n *Notifier) sendListingWithPhoto(ctx context.Context, chatID string, listing model.Listing) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID %q: %w", chatID, err)
	}

	caption := notifier.FormatListing(listing)

	if len([]rune(caption)) > maxCaptionLen {
		_, err = n.bot.SendPhoto(ctx, &tgbot.SendPhotoParams{
			ChatID: id,
			Photo:  &tgmodels.InputFileString{Data: listing.ImageURL},
		})
		if err != nil {
			n.logger.Warn("sendPhoto failed, falling back to text", "chat_id", chatID, "error", err)
			return n.sendMessage(ctx, chatID, caption)
		}
		return n.sendMessage(ctx, chatID, caption)
	}

	_, err = n.bot.SendPhoto(ctx, &tgbot.SendPhotoParams{
		ChatID:    id,
		Photo:     &tgmodels.InputFileString{Data: listing.ImageURL},
		Caption:   caption,
		ParseMode: tgmodels.ParseModeMarkdown,
	})
	if err != nil {
		n.logger.Warn("sendPhoto failed, falling back to text", "chat_id", chatID, "error", err)
		return n.sendMessage(ctx, chatID, caption)
	}

	n.logger.Info("sent telegram photo", "chat_id", chatID)
	return nil
}

func (n *Notifier) sendMessage(ctx context.Context, chatID string, text string) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID %q: %w", chatID, err)
	}

	chunks := splitMessage(text, maxMessageLen)
	for _, chunk := range chunks {
		_, err = n.bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    id,
			Text:      chunk,
			ParseMode: tgmodels.ParseModeMarkdown,
		})
		if err != nil {
			errMsg := strings.ToLower(err.Error())
			if strings.Contains(errMsg, "bot was blocked by the user") ||
				strings.Contains(errMsg, "user is deactivated") {
				return fmt.Errorf("%w: %v", notifier.ErrRecipientBlocked, err)
			}
			return fmt.Errorf("telegram sendMessage: %w", err)
		}
	}

	n.logger.Info("sent telegram message", "chat_id", chatID, "chunks", len(chunks))
	return nil
}

func splitMessage(text string, limit int) []string {
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

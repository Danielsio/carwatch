package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/dsionov/carwatch/internal/model"
	"github.com/dsionov/carwatch/internal/notifier"
)

type TelegramNotifier struct {
	token  string
	client *http.Client
	logger *slog.Logger
}

func New(token string, logger *slog.Logger) *TelegramNotifier {
	return &TelegramNotifier{
		token: token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (t *TelegramNotifier) Connect(_ context.Context) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", t.token)
	resp, err := t.client.Get(url)
	if err != nil {
		return fmt.Errorf("telegram getMe: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram getMe: status %d: %s", resp.StatusCode, body)
	}

	t.logger.Info("telegram notifier connected")
	return nil
}

func (t *TelegramNotifier) Notify(_ context.Context, chatID string, listings []model.Listing) error {
	msg := notifier.FormatBatch(listings)

	payload := map[string]any{
		"chat_id":    chatID,
		"text":       msg,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)
	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram sendMessage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendMessage: status %d: %s", resp.StatusCode, respBody)
	}

	t.logger.Info("sent telegram message", "chat_id", chatID, "listings", len(listings))
	return nil
}

func (t *TelegramNotifier) Disconnect() error {
	return nil
}

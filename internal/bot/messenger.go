package bot

import (
	"context"
	"os"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

type messenger interface {
	SendMessage(ctx context.Context, chatID int64, text string, parseMode string, kb *tgmodels.InlineKeyboardMarkup) error
	SendPhoto(ctx context.Context, chatID int64, photoPath string, caption string, parseMode string, kb *tgmodels.InlineKeyboardMarkup) error
	AnswerCallback(ctx context.Context, callbackID string) error
}

type telegramMessenger struct {
	bot *tgbot.Bot
}

func (t *telegramMessenger) SendMessage(ctx context.Context, chatID int64, text string, parseMode string, kb *tgmodels.InlineKeyboardMarkup) error {
	params := &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}
	if parseMode != "" {
		params.ParseMode = tgmodels.ParseMode(parseMode)
	}
	if kb != nil {
		params.ReplyMarkup = kb
	}
	_, err := t.bot.SendMessage(ctx, params)
	return err
}

func (t *telegramMessenger) SendPhoto(ctx context.Context, chatID int64, photoPath string, caption string, parseMode string, kb *tgmodels.InlineKeyboardMarkup) error {
	f, err := os.Open(photoPath)
	if err != nil {
		return err
	}
	defer f.Close()

	params := &tgbot.SendPhotoParams{
		ChatID:  chatID,
		Photo:   &tgmodels.InputFileUpload{Filename: "carwatch-logo.png", Data: f},
		Caption: caption,
	}
	if parseMode != "" {
		params.ParseMode = tgmodels.ParseMode(parseMode)
	}
	if kb != nil {
		params.ReplyMarkup = kb
	}
	_, err = t.bot.SendPhoto(ctx, params)
	return err
}

func (t *telegramMessenger) AnswerCallback(ctx context.Context, callbackID string) error {
	_, err := t.bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
	})
	return err
}

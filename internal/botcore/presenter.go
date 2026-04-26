package botcore

import "context"

type Choice struct {
	Label string
	Data  string
}

type Presenter interface {
	SendText(ctx context.Context, chatID int64, text string) error
	SendMarkdown(ctx context.Context, chatID int64, text string) error
	SendChoices(ctx context.Context, chatID int64, prompt string, choices [][]Choice) error
	SendConfirmation(ctx context.Context, chatID int64, summary string, choices [][]Choice) error
	SendPhoto(ctx context.Context, chatID int64, photoURL string, caption string) error
}

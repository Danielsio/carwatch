package cwlog

import "context"

type ctxKey int

const (
	keyCycleID ctxKey = iota
	keySearchID
	keyChatID
	keyRequestID
	keyComponent
)

func WithCycleID(ctx context.Context, id uint64) context.Context {
	return context.WithValue(ctx, keyCycleID, id)
}

func WithSearchID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, keySearchID, id)
}

func WithChatID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, keyChatID, id)
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

func WithComponent(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, keyComponent, name)
}

// WithSearch sets both search_id and chat_id in a single call.
func WithSearch(ctx context.Context, searchID, chatID int64) context.Context {
	ctx = context.WithValue(ctx, keySearchID, searchID)
	ctx = context.WithValue(ctx, keyChatID, chatID)
	return ctx
}

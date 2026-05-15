package bot

import (
	"context"
	"strings"

	"github.com/dsionov/carwatch/internal/locale"
)

type callbackHandler func(b *Bot, ctx context.Context, chatID int64, data string)

type callbackPrefixRoute struct {
	prefix string
	h      callbackHandler
}

var (
	callbackExact    map[string]callbackHandler
	callbackPrefixes []callbackPrefixRoute
)

func init() {
	callbackExact = map[string]callbackHandler{
		cbSourceDone: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onSourceDone(ctx, chatID)
		},
		cbMfrSearch: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onMfrSearch(ctx, chatID)
		},
		cbMdlSearch: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onMdlSearch(ctx, chatID)
		},
		cbSkipPriceMin: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onSkipPriceMin(ctx, chatID)
		},
		cbSkipKeywords: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onSkipKeywords(ctx, chatID)
		},
		cbSkipExcludeKeys: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onSkipExcludeKeys(ctx, chatID)
		},
		cbConfirm: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onConfirm(ctx, chatID)
		},
		cbEdit: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onEditRestart(ctx, chatID)
		},
		cbCancel: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onCancelCallback(ctx, chatID)
		},
		cbDigestOn: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onDigestOn(ctx, chatID)
		},
		cbDigestOff: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onDigestOff(ctx, chatID)
		},
		cbLangHe: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onLanguageSwitch(ctx, chatID, locale.Hebrew)
		},
		cbLangEn: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onLanguageSwitch(ctx, chatID, locale.English)
		},
		cbQuickStart: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onQuickStart(ctx, chatID)
		},
		cbHiddenClear: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onClearHidden(ctx, chatID)
		},
		cbDailyDigestOn: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onDailyDigestOn(ctx, chatID)
		},
		cbDailyDigestOff: func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onDailyDigestOff(ctx, chatID)
		},
		"watch": func(b *Bot, ctx context.Context, chatID int64, _ string) {
			b.onWatchFromCallback(ctx, chatID)
		},
		"noop": func(*Bot, context.Context, int64, string) {},
	}

	callbackPrefixes = []callbackPrefixRoute{
		{cbPrefixSource, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onLegacySourceSelected(ctx, chatID, strings.TrimPrefix(data, cbPrefixSource))
		}},
		{cbSourceToggle, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onSourceToggle(ctx, chatID, data)
		}},
		{cbMfrPage, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onMfrPage(ctx, chatID, data)
		}},
		{cbPrefixMfr, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onManufacturerSelected(ctx, chatID, data)
		}},
		{cbMdlPage, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onMdlPage(ctx, chatID, data)
		}},
		{cbPrefixModel, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onModelSelected(ctx, chatID, data)
		}},
		{cbPrefixGearBox, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onGearBoxSelected(ctx, chatID, data)
		}},
		{cbPrefixEngine, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onEngineSelected(ctx, chatID, data)
		}},
		{cbPrefixMaxKm, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onMaxKmSelected(ctx, chatID, data)
		}},
		{cbPrefixMaxHand, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onMaxHandSelected(ctx, chatID, data)
		}},
		{cbDeleteSearch, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onDeleteSearch(ctx, chatID, data)
		}},
		{cbPrefixShareCopy, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onShareCopy(ctx, chatID, data)
		}},
		{cbDigestInterval, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onDigestInterval(ctx, chatID, data)
		}},
		{cbHistoryPage, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onHistoryPage(ctx, chatID, data)
		}},
		{cbPrefixSave, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onSaveListing(ctx, chatID, data)
		}},
		{cbPrefixHide, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onHideListing(ctx, chatID, data)
		}},
		{cbSavedPage, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onSavedPage(ctx, chatID, data)
		}},
		{cbHiddenPage, func(b *Bot, ctx context.Context, chatID int64, data string) {
			b.onHiddenPage(ctx, chatID, data)
		}},
	}
}

func (b *Bot) dispatchCallbackData(ctx context.Context, chatID int64, data string) {
	if h, ok := callbackExact[data]; ok {
		h(b, ctx, chatID, data)
		return
	}
	for _, pr := range callbackPrefixes {
		if strings.HasPrefix(data, pr.prefix) {
			pr.h(b, ctx, chatID, data)
			return
		}
	}
}

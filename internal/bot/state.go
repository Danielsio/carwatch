package bot

import (
	"context"

	"github.com/dsionov/carwatch/internal/botcore"
	"github.com/dsionov/carwatch/internal/locale"
)

const (
	StateIdle               = botcore.StateIdle
	StateAskSource          = botcore.StateAskSource
	StateAskManufacturer    = botcore.StateAskManufacturer
	StateSearchManufacturer = botcore.StateSearchManufacturer
	StateAskModel           = botcore.StateAskModel
	StateSearchModel        = botcore.StateSearchModel
	StateAskYearMin         = botcore.StateAskYearMin
	StateAskYearMax         = botcore.StateAskYearMax
	StateAskPriceMax        = botcore.StateAskPriceMax
	StateAskPriceMin        = botcore.StateAskPriceMin
	StateAskGearBox         = botcore.StateAskGearBox
	StateAskEngine          = botcore.StateAskEngine
	StateAskMaxKm           = botcore.StateAskMaxKm
	StateAskMaxHand         = botcore.StateAskMaxHand
	StateAskKeywords        = botcore.StateAskKeywords
	StateAskExcludeKeys     = botcore.StateAskExcludeKeys
	StateConfirm            = botcore.StateConfirm
)

type WizardData = botcore.WizardData

func (b *Bot) expectStateOrNotify(ctx context.Context, chatID int64, expected string) bool {
	lang := b.getUserLang(ctx, chatID)
	user, err := b.users.GetUser(ctx, chatID)
	if err != nil {
		b.logger.Error("expectState: get user failed", "chat_id", chatID, "error", err)
		b.send(ctx, chatID, locale.T(lang, "error_generic"))
		return false
	}
	if user == nil || user.State != expected {
		b.send(ctx, chatID, locale.T(lang, "callback_expired"))
		return false
	}
	return true
}

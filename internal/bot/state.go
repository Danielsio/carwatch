package bot

import (
	"context"

	"github.com/dsionov/carwatch/internal/botcore"
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

func (b *Bot) expectState(ctx context.Context, chatID int64, expected string) bool {
	user, err := b.users.GetUser(ctx, chatID)
	if err != nil || user == nil || user.State != expected {
		return false
	}
	return true
}

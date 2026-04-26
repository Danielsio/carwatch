package bot

import "github.com/dsionov/carwatch/internal/botcore"

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
	StateAskEngine          = botcore.StateAskEngine
	StateAskMaxKm           = botcore.StateAskMaxKm
	StateAskMaxHand         = botcore.StateAskMaxHand
	StateAskKeywords        = botcore.StateAskKeywords
	StateAskExcludeKeys     = botcore.StateAskExcludeKeys
	StateConfirm            = botcore.StateConfirm
)

type WizardData = botcore.WizardData

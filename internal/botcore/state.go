package botcore

const (
	StateIdle               = "idle"
	StateAskSource          = "ask_source"
	StateAskManufacturer    = "ask_manufacturer"
	StateSearchManufacturer = "search_manufacturer"
	StateAskModel           = "ask_model"
	StateSearchModel        = "search_model"
	StateAskYearMin         = "ask_year_min"
	StateAskYearMax         = "ask_year_max"
	StateAskPriceMax        = "ask_price_max"
	StateAskEngine          = "ask_engine"
	StateAskMaxKm           = "ask_max_km"
	StateAskMaxHand         = "ask_max_hand"
	StateAskKeywords        = "ask_keywords"
	StateAskExcludeKeys     = "ask_exclude_keys"
	StateConfirm            = "confirm"
)

type WizardData struct {
	Source           string `json:"source,omitempty"`
	Manufacturer     int    `json:"manufacturer,omitempty"`
	ManufacturerName string `json:"manufacturer_name,omitempty"`
	Model            int    `json:"model,omitempty"`
	ModelName        string `json:"model_name,omitempty"`
	YearMin          int    `json:"year_min,omitempty"`
	YearMax          int    `json:"year_max,omitempty"`
	PriceMax         int    `json:"price_max,omitempty"`
	EngineMinCC      int    `json:"engine_min_cc,omitempty"`
	MaxKm            int    `json:"max_km,omitempty"`
	MaxHand          int    `json:"max_hand,omitempty"`
	Keywords         string `json:"keywords,omitempty"`
	ExcludeKeys      string `json:"exclude_keys,omitempty"`
	EditSearchID     int64  `json:"edit_search_id,omitempty"`
}

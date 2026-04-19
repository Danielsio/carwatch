package bot

const (
	StateIdle            = "idle"
	StateAskSource       = "ask_source"
	StateAskManufacturer = "ask_manufacturer"
	StateAskModel        = "ask_model"
	StateAskYearMin      = "ask_year_min"
	StateAskYearMax      = "ask_year_max"
	StateAskPriceMax     = "ask_price_max"
	StateAskEngine       = "ask_engine"
	StateConfirm         = "confirm"
)

type WizardData struct {
	Source          string `json:"source,omitempty"`
	Manufacturer    int    `json:"manufacturer,omitempty"`
	ManufacturerName string `json:"manufacturer_name,omitempty"`
	Model           int    `json:"model,omitempty"`
	ModelName       string `json:"model_name,omitempty"`
	YearMin         int    `json:"year_min,omitempty"`
	YearMax         int    `json:"year_max,omitempty"`
	PriceMax        int    `json:"price_max,omitempty"`
	EngineMinCC     int    `json:"engine_min_cc,omitempty"`
}

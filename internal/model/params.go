package model

// SourceParams defines the search parameters sent to listing sources.
type SourceParams struct {
	Manufacturer int
	Model        int
	YearMin      int
	YearMax      int
	PriceMin     int
	PriceMax     int
	Page         int
}

// FilterCriteria defines the criteria used to filter raw listings
// after they are fetched from a source.
type FilterCriteria struct {
	YearMin     int
	YearMax     int
	PriceMax    int
	EngineMinCC float64
	EngineMaxCC float64
	MaxKm       int
	MaxHand     int
	Keywords    []string
	ExcludeKeys []string
}

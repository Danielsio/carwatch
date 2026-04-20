package catalog

type Entry struct {
	ID   int
	Name string
}

type Catalog interface {
	Manufacturers() []Entry
	Models(manufacturerID int) []Entry
	ManufacturerName(id int) string
	ModelName(manufacturerID, modelID int) string
}

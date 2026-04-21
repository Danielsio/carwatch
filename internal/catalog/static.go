package catalog

type staticCatalog struct {
	mfrs   []Entry
	models map[int][]Entry
}

func NewStatic() Catalog {
	return &staticCatalog{
		mfrs:   defaultManufacturers,
		models: defaultModels,
	}
}

func (c *staticCatalog) Manufacturers() []Entry {
	return c.mfrs
}

func (c *staticCatalog) Models(manufacturerID int) []Entry {
	return c.models[manufacturerID]
}

func (c *staticCatalog) ManufacturerName(id int) string {
	for _, m := range c.mfrs {
		if m.ID == id {
			return m.Name
		}
	}
	return "Unknown"
}

func (c *staticCatalog) ModelName(manufacturerID, modelID int) string {
	for _, m := range c.models[manufacturerID] {
		if m.ID == modelID {
			return m.Name
		}
	}
	return "Unknown"
}

var defaultManufacturers = []Entry{
	{ID: 1, Name: "Audi"},
	{ID: 7, Name: "BMW"},
	{ID: 17, Name: "Honda"},
	{ID: 18, Name: "Volvo"},
	{ID: 19, Name: "Toyota"},
	{ID: 21, Name: "Hyundai"},
	{ID: 27, Name: "Mazda"},
	{ID: 31, Name: "Mercedes"},
	{ID: 32, Name: "Nissan"},
	{ID: 40, Name: "Skoda"},
	{ID: 41, Name: "Volkswagen"},
	{ID: 48, Name: "Kia"},
	{ID: 51, Name: "Renault"},
	{ID: 62, Name: "Tesla"},
}

var defaultModels = map[int][]Entry{
	1: {
		{ID: 10004, Name: "A3"},
		{ID: 10005, Name: "A4"},
		{ID: 10007, Name: "A6"},
		{ID: 10012, Name: "Q3"},
		{ID: 10013, Name: "Q5"},
	},
	7: {
		{ID: 10095, Name: "1 Series"},
		{ID: 10097, Name: "3 Series"},
		{ID: 10099, Name: "5 Series"},
		{ID: 10111, Name: "X1"},
		{ID: 10113, Name: "X3"},
		{ID: 10116, Name: "X5"},
	},
	17: {
		{ID: 10182, Name: "Civic"},
		{ID: 10183, Name: "CR-V"},
		{ID: 10186, Name: "HR-V"},
		{ID: 10188, Name: "Jazz"},
	},
	18: {
		{ID: 10203, Name: "S60"},
		{ID: 10207, Name: "V40"},
		{ID: 10212, Name: "XC40"},
		{ID: 10213, Name: "XC60"},
		{ID: 10215, Name: "XC90"},
	},
	19: {
		{ID: 10225, Name: "C-HR"},
		{ID: 10226, Name: "Corolla"},
		{ID: 10238, Name: "RAV4"},
		{ID: 10247, Name: "Yaris"},
		{ID: 11150, Name: "Corolla Cross"},
	},
	21: {
		{ID: 10259, Name: "Accent"},
		{ID: 10263, Name: "Elantra"},
		{ID: 10272, Name: "i10"},
		{ID: 10273, Name: "i20"},
		{ID: 10276, Name: "i30"},
		{ID: 10279, Name: "Ioniq"},
		{ID: 10288, Name: "Sonata"},
		{ID: 10291, Name: "Tucson"},
	},
	27: {
		{ID: 10331, Name: "2"},
		{ID: 10332, Name: "3"},
		{ID: 10335, Name: "6"},
		{ID: 10340, Name: "CX-3"},
		{ID: 10341, Name: "CX-30"},
		{ID: 10342, Name: "CX-5"},
	},
	31: {
		{ID: 10389, Name: "A-Class"},
		{ID: 10394, Name: "C-Class"},
		{ID: 10397, Name: "CLA"},
		{ID: 10401, Name: "E-Class"},
		{ID: 10407, Name: "GLC"},
	},
	32: {
		{ID: 10433, Name: "Juke"},
		{ID: 10438, Name: "Micra"},
		{ID: 10449, Name: "Qashqai"},
		{ID: 10457, Name: "X-Trail"},
	},
	40: {
		{ID: 10541, Name: "Fabia"},
		{ID: 10545, Name: "Karoq"},
		{ID: 10546, Name: "Kodiaq"},
		{ID: 10547, Name: "Octavia"},
		{ID: 10551, Name: "Superb"},
		{ID: 11568, Name: "Enyaq"},
	},
	41: {
		{ID: 10562, Name: "Golf"},
		{ID: 10571, Name: "Polo"},
		{ID: 10574, Name: "Tiguan"},
	},
	48: {
		{ID: 10698, Name: "Ceed"},
		{ID: 10708, Name: "Niro"},
		{ID: 10711, Name: "Picanto"},
		{ID: 10720, Name: "Sportage"},
	},
	51: {
		{ID: 10750, Name: "Captur"},
		{ID: 10751, Name: "Clio"},
		{ID: 10762, Name: "Megane"},
	},
	62: {
		{ID: 10846, Name: "Model 3"},
		{ID: 11942, Name: "Model Y"},
	},
}

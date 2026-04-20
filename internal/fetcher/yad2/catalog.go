package yad2

type CatalogEntry struct {
	ID   int
	Name string
}

var manufacturers = []CatalogEntry{
	{ID: 19, Name: "Hyundai"},
	{ID: 20, Name: "Honda"},
	{ID: 27, Name: "Mazda"},
	{ID: 28, Name: "Mercedes"},
	{ID: 31, Name: "Nissan"},
	{ID: 35, Name: "Toyota"},
	{ID: 36, Name: "Volkswagen"},
	{ID: 39, Name: "BMW"},
	{ID: 40, Name: "Audi"},
	{ID: 43, Name: "Kia"},
	{ID: 44, Name: "Volvo"},
	{ID: 47, Name: "Skoda"},
	{ID: 50, Name: "Renault"},
	{ID: 80, Name: "Tesla"},
}

var modelsByManufacturer = map[int][]CatalogEntry{
	19: { // Hyundai
		{ID: 10102, Name: "i10"},
		{ID: 10103, Name: "i20"},
		{ID: 10104, Name: "i25"},
		{ID: 10105, Name: "i30"},
		{ID: 10106, Name: "i35"},
		{ID: 10107, Name: "i40"},
		{ID: 10768, Name: "Ioniq"},
		{ID: 10824, Name: "Ioniq 5"},
		{ID: 10769, Name: "Kona"},
		{ID: 10186, Name: "Tucson"},
		{ID: 10187, Name: "Santa Fe"},
	},
	20: { // Honda
		{ID: 10192, Name: "Civic"},
		{ID: 10197, Name: "Jazz"},
		{ID: 10195, Name: "CR-V"},
		{ID: 10196, Name: "HR-V"},
	},
	27: { // Mazda
		{ID: 10331, Name: "2"},
		{ID: 10332, Name: "3"},
		{ID: 10333, Name: "6"},
		{ID: 10700, Name: "CX-3"},
		{ID: 10701, Name: "CX-5"},
		{ID: 10771, Name: "CX-30"},
		{ID: 10826, Name: "CX-60"},
	},
	28: { // Mercedes
		{ID: 10342, Name: "A-Class"},
		{ID: 10344, Name: "C-Class"},
		{ID: 10346, Name: "E-Class"},
		{ID: 10735, Name: "CLA"},
		{ID: 10348, Name: "GLC"},
		{ID: 10350, Name: "GLE"},
	},
	31: { // Nissan
		{ID: 10382, Name: "Micra"},
		{ID: 10390, Name: "Qashqai"},
		{ID: 10391, Name: "X-Trail"},
		{ID: 10386, Name: "Juke"},
		{ID: 10810, Name: "Leaf"},
	},
	35: { // Toyota
		{ID: 10471, Name: "Corolla"},
		{ID: 10480, Name: "Yaris"},
		{ID: 10702, Name: "C-HR"},
		{ID: 10475, Name: "RAV4"},
		{ID: 10473, Name: "Camry"},
		{ID: 10774, Name: "Corolla Cross"},
	},
	36: { // Volkswagen
		{ID: 10488, Name: "Golf"},
		{ID: 10494, Name: "Polo"},
		{ID: 10497, Name: "Tiguan"},
		{ID: 10776, Name: "T-Cross"},
		{ID: 10777, Name: "T-Roc"},
		{ID: 10817, Name: "ID.3"},
		{ID: 10818, Name: "ID.4"},
	},
	39: { // BMW
		{ID: 10031, Name: "1 Series"},
		{ID: 10032, Name: "2 Series"},
		{ID: 10033, Name: "3 Series"},
		{ID: 10035, Name: "5 Series"},
		{ID: 10038, Name: "X1"},
		{ID: 10039, Name: "X3"},
		{ID: 10040, Name: "X5"},
	},
	40: { // Audi
		{ID: 10005, Name: "A3"},
		{ID: 10006, Name: "A4"},
		{ID: 10008, Name: "A6"},
		{ID: 10015, Name: "Q3"},
		{ID: 10016, Name: "Q5"},
		{ID: 10819, Name: "Q4 e-tron"},
	},
	43: { // Kia
		{ID: 10247, Name: "Picanto"},
		{ID: 10253, Name: "Rio"},
		{ID: 10245, Name: "Ceed"},
		{ID: 10254, Name: "Sportage"},
		{ID: 10770, Name: "Niro"},
		{ID: 10825, Name: "EV6"},
		{ID: 10829, Name: "EV9"},
	},
	44: { // Volvo
		{ID: 10505, Name: "XC40"},
		{ID: 10506, Name: "XC60"},
		{ID: 10507, Name: "XC90"},
		{ID: 10503, Name: "S60"},
		{ID: 10504, Name: "V40"},
	},
	47: { // Skoda
		{ID: 10431, Name: "Fabia"},
		{ID: 10433, Name: "Octavia"},
		{ID: 10434, Name: "Superb"},
		{ID: 10706, Name: "Karoq"},
		{ID: 10707, Name: "Kodiaq"},
		{ID: 10830, Name: "Enyaq"},
	},
	50: { // Renault
		{ID: 10412, Name: "Clio"},
		{ID: 10414, Name: "Megane"},
		{ID: 10416, Name: "Captur"},
		{ID: 10417, Name: "Kadjar"},
	},
	80: { // Tesla
		{ID: 10808, Name: "Model 3"},
		{ID: 10809, Name: "Model Y"},
	},
}

func Manufacturers() []CatalogEntry {
	return manufacturers
}

func Models(manufacturerID int) []CatalogEntry {
	return modelsByManufacturer[manufacturerID]
}

func ManufacturerName(id int) string {
	for _, m := range manufacturers {
		if m.ID == id {
			return m.Name
		}
	}
	return "Unknown"
}

func ModelName(manufacturerID, modelID int) string {
	for _, m := range modelsByManufacturer[manufacturerID] {
		if m.ID == modelID {
			return m.Name
		}
	}
	return "Unknown"
}

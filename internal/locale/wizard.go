package locale

var heWizard = map[string]string{
	"wizard_source_prompt": "באילו אתרים לחפש? (בחר אחד או שניהם)\n\n⏱ ההגדרה תפוג אחרי 30 דקות של חוסר פעילות.",
	"wizard_source_empty":  "אנא בחר לפחות אתר אחד.",
	"wizard_start_over":    "בוא נתחיל מחדש. באילו אתרים?",

	// wizard - manufacturer
	"wizard_mfr_prompt":  "איזה יצרן אתה מחפש?",
	"wizard_mfr_search":  "הקלד את שם היצרן:",
	"wizard_mfr_results": "תוצאות חיפוש:",

	// wizard - model
	"wizard_model_prompt":  "איזה דגם %s?",
	"wizard_model_search":  "הקלד את שם דגם ה-%s:",
	"wizard_model_results": "תוצאות חיפוש:",

	// wizard - year
	"wizard_year_min":       "משנת כמה? (למשל 2018)",
	"wizard_year_max":       "עד שנת כמה? (למשל 2024)",
	"wizard_year_invalid":   "אנא הזן שנה תקינה (%d–%d).",
	"wizard_year_min_error": "חייב להיות >= %d. נסה שוב.",

	// wizard - price
	"wizard_price_prompt":  "מחיר מקסימלי ב-₪? (למשל 150000)",
	"wizard_price_invalid": "אנא הזן מחיר תקין (1,000–10,000,000).",

	// wizard - price min
	"wizard_price_min_prompt":      "מחיר מינימלי ב-₪? (אופציונלי — הקלד מספר או דלג)",
	"wizard_price_min_invalid":     "אנא הזן מחיר תקין (0–10,000,000).",
	"wizard_price_min_exceeds_max": "מחיר מינימלי לא יכול לעלות על המקסימלי. נסה שוב.",

	// wizard - gearbox
	"wizard_gearbox_prompt": "סוג תיבת הילוכים?",

	// wizard - engine
	"wizard_engine_prompt": "נפח מנוע מינימלי?",

	// wizard - km
	"wizard_km_prompt": "קילומטראז׳ מקסימלי?",

	// wizard - hand
	"wizard_hand_prompt": "יד מקסימלית?",

	// wizard - keywords
	"wizard_keywords_prompt":     "מילות מפתח לחיפוש בתיאור? (מופרדות בפסיק, או הקלד 'דלג')\nלדוגמה: אוטומטי, שמור",
	"wizard_exclude_keys_prompt": "מילות מפתח להחרגה מהתיאור? (מופרדות בפסיק, או הקלד 'דלג')\nלדוגמה: תאונה, חורף",
	"wizard_keywords_skip":       "דלג",
	"wizard_keywords_too_long":   "מילות מפתח ארוכות מדי (מקסימום %d תווים). אנא קצר ונסה שוב.",

	// wizard - confirm
	"wizard_confirm_summary": "*החיפוש שלך:*\n" +
		"מקור: %s\n" +
		"רכב: %s %s\n" +
		"שנים: %d–%d\n" +
		"מחיר מקסימלי: %s ₪\n" +
		"מנוע: %s\n" +
		"ק״מ מקסימלי: %s\n" +
		"יד מקסימלית: %s",
	"wizard_confirm_price_min":    "\nמחיר מינימלי: %s ₪",
	"wizard_confirm_gearbox":      "\nתיבת הילוכים: %s",
	"wizard_confirm_keywords":     "\nמילות מפתח: %s",
	"wizard_confirm_exclude_keys": "\nמילות החרגה: %s",
	"wizard_search_saved":         "חיפוש #%d נשמר! בודק %s עכשיו...\n\nהשתמש ב /list כדי לראות את החיפושים שלך.",
	"wizard_search_updated":       "חיפוש #%d עודכן!\n\nהשתמש ב /list כדי לראות את החיפושים שלך.",
	"wizard_save_failed":          "שמירת החיפוש נכשלה. אנא נסה שוב.",
	"wizard_session_expired":      "הסשן פג. השתמש ב /watch כדי להתחיל חיפוש חדש.",

	// edit diff
	"edit_diff_header":     "*שינויים:*",
	"edit_diff_no_changes": "אין שינויים.",
	"edit_diff_source":     "מקור",
	"edit_diff_car":        "רכב",
	"edit_diff_year_min":   "שנה מינימלית",
	"edit_diff_year_max":   "שנה מקסימלית",
	"edit_diff_price_max":  "מחיר מקסימלי",
	"edit_diff_price_min":  "מחיר מינימלי",
	"edit_diff_gearbox":    "תיבת הילוכים",
	"edit_diff_engine":     "מנוע",
	"edit_diff_km":         "ק\"מ מקסימלי",
	"edit_diff_hand":       "יד מקסימלית",
	"edit_diff_keywords":   "מילות מפתח",
	"edit_diff_exclude":    "מילות החרגה",

	// /watch
}

var enWizard = map[string]string{
	"wizard_source_prompt": "Which marketplaces do you want to search? (select one or both)\n\n⏱ This setup expires after 30 minutes of inactivity.",
	"wizard_source_empty":  "Please select at least one marketplace.",
	"wizard_start_over":    "Let's start over. Which marketplaces?",

	// wizard - manufacturer
	"wizard_mfr_prompt":  "What manufacturer are you looking for?",
	"wizard_mfr_search":  "Type the manufacturer name:",
	"wizard_mfr_results": "Search results:",

	// wizard - model
	"wizard_model_prompt":  "Which %s model?",
	"wizard_model_search":  "Type the %s model name:",
	"wizard_model_results": "Search results:",

	// wizard - year
	"wizard_year_min":       "From which year? (e.g. 2018)",
	"wizard_year_max":       "Until which year? (e.g. 2024)",
	"wizard_year_invalid":   "Please enter a valid year (%d–%d).",
	"wizard_year_min_error": "Must be >= %d. Try again.",

	// wizard - price
	"wizard_price_prompt":  "Max price in NIS? (e.g. 150000)",
	"wizard_price_invalid": "Please enter a valid price (1,000–10,000,000).",

	// wizard - price min
	"wizard_price_min_prompt":      "Minimum price in NIS? (optional -- type a number or skip)",
	"wizard_price_min_invalid":     "Please enter a valid price (0-10,000,000).",
	"wizard_price_min_exceeds_max": "Minimum price cannot exceed maximum price. Try again.",

	// wizard - gearbox
	"wizard_gearbox_prompt": "Gearbox type?",

	// wizard - engine
	"wizard_engine_prompt": "Minimum engine size?",

	// wizard - km
	"wizard_km_prompt": "Maximum kilometers?",

	// wizard - hand
	"wizard_hand_prompt": "Maximum ownership hand?",

	// wizard - keywords
	"wizard_keywords_prompt":     "Any keywords to require in the description? (comma-separated, or type 'skip')\nExample: automatic, well-kept",
	"wizard_exclude_keys_prompt": "Any keywords to exclude? (comma-separated, or type 'skip')\nExample: accident, damaged",
	"wizard_keywords_skip":       "skip",
	"wizard_keywords_too_long":   "Keywords are too long (max %d characters). Please shorten and try again.",

	// wizard - confirm
	"wizard_confirm_summary": "*Your search:*\n" +
		"Source: %s\n" +
		"Car: %s %s\n" +
		"Year: %d–%d\n" +
		"Max price: %s NIS\n" +
		"Engine: %s\n" +
		"Max km: %s\n" +
		"Max hand: %s",
	"wizard_confirm_price_min":    "\nMin price: %s NIS",
	"wizard_confirm_gearbox":      "\nGearbox: %s",
	"wizard_confirm_keywords":     "\nKeywords: %s",
	"wizard_confirm_exclude_keys": "\nExclude: %s",
	"wizard_search_saved":         "Search #%d saved! Checking %s now...\n\nUse /list to see your searches.",
	"wizard_search_updated":       "Search #%d updated!\n\nUse /list to see your searches.",
	"wizard_save_failed":          "Failed to save search. Please try again.",
	"wizard_session_expired":      "Session expired. Use /watch to start a new search.",

	// edit diff
	"edit_diff_header":     "*Changes:*",
	"edit_diff_no_changes": "No changes detected.",
	"edit_diff_source":     "Source",
	"edit_diff_car":        "Car",
	"edit_diff_year_min":   "Year min",
	"edit_diff_year_max":   "Year max",
	"edit_diff_price_max":  "Max price",
	"edit_diff_price_min":  "Min price",
	"edit_diff_gearbox":    "Gearbox",
	"edit_diff_engine":     "Engine",
	"edit_diff_km":         "Max km",
	"edit_diff_hand":       "Max hand",
	"edit_diff_keywords":   "Keywords",
	"edit_diff_exclude":    "Exclude",

	// /watch
}

package locale

import "fmt"

type Lang string

const (
	Hebrew  Lang = "he"
	English Lang = "en"
)

func T(lang Lang, key string) string {
	if m, ok := translations[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := translations[English][key]; ok {
		return s
	}
	return key
}

func Tf(lang Lang, key string, args ...any) string {
	return fmt.Sprintf(T(lang, key), args...)
}

var translations = map[Lang]map[string]string{
	Hebrew:  he,
	English: en,
}

var he = map[string]string{
	// /start
	"welcome": "ברוכים הבאים ל-*CarWatch*! אני עוקב אחרי מודעות רכב ביד2 ו-WinWin ושולח לך התראות כשמופיעים רכבים חדשים שמתאימים לחיפוש שלך.\n\n" +
		"השתמש ב /watch כדי להגדיר חיפוש חדש.\n" +
		"השתמש ב /list כדי לראות את החיפושים הפעילים שלך.\n" +
		"השתמש ב /help לרשימת כל הפקודות.",

	// /help
	"help": "*פקודות CarWatch:*\n\n" +
		"/watch — הגדר חיפוש רכב חדש\n" +
		"/list — הצג חיפושים פעילים\n" +
		"/edit <מספר> — ערוך חיפוש קיים\n" +
		"/history — צפה בהיסטוריית התאמות\n" +
		"/pause <מספר> — השהה חיפוש\n" +
		"/resume <מספר> — חדש חיפוש מושהה\n" +
		"/stop <מספר> — מחק חיפוש\n" +
		"/share <מספר> — שתף חיפוש בקישור\n" +
		"/saved — צפה ברכבים שמורים\n" +
		"/hidden — צפה ברכבים מוסתרים\n" +
		"/digest — התראות וסיכום שוק יומי\n" +
		"/upgrade — שדרוג לפרימיום\n" +
		"/language — שנה שפה\n" +
		"/settings — הצג הגדרות\n" +
		"/cancel — בטל אשף נוכחי\n" +
		"/help — הצג הודעה זו",

	// wizard - source
	"wizard_source_prompt":  "באילו אתרים לחפש? (בחר אחד או שניהם)",
	"wizard_source_empty":   "אנא בחר לפחות אתר אחד.",
	"wizard_start_over":     "בוא נתחיל מחדש. באילו אתרים?",

	// wizard - manufacturer
	"wizard_mfr_prompt":     "איזה יצרן אתה מחפש?",
	"wizard_mfr_search":     "הקלד את שם היצרן:",
	"wizard_mfr_results":    "תוצאות חיפוש:",

	// wizard - model
	"wizard_model_prompt":   "איזה דגם %s?",
	"wizard_model_search":   "הקלד את שם דגם ה-%s:",
	"wizard_model_results":  "תוצאות חיפוש:",

	// wizard - year
	"wizard_year_min":       "משנת כמה? (למשל 2018)",
	"wizard_year_max":       "עד שנת כמה? (למשל 2024)",
	"wizard_year_invalid":   "אנא הזן שנה תקינה (%d–%d).",
	"wizard_year_min_error": "חייב להיות >= %d. נסה שוב.",

	// wizard - price
	"wizard_price_prompt":   "מחיר מקסימלי ב-₪? (למשל 150000)",
	"wizard_price_invalid":  "אנא הזן מחיר תקין (1,000–10,000,000).",

	// wizard - engine
	"wizard_engine_prompt":  "נפח מנוע מינימלי?",

	// wizard - km
	"wizard_km_prompt":      "קילומטראז׳ מקסימלי?",

	// wizard - hand
	"wizard_hand_prompt":    "יד מקסימלית?",

	// wizard - keywords
	"wizard_keywords_prompt":      "מילות מפתח לחיפוש בתיאור? (מופרדות בפסיק, או הקלד 'דלג')\nלדוגמה: אוטומטי, שמור",
	"wizard_exclude_keys_prompt":  "מילות מפתח להחרגה מהתיאור? (מופרדות בפסיק, או הקלד 'דלג')\nלדוגמה: תאונה, חורף",
	"wizard_keywords_skip":        "דלג",

	// wizard - confirm
	"wizard_confirm_summary": "*החיפוש שלך:*\n" +
		"מקור: %s\n" +
		"רכב: %s %s\n" +
		"שנים: %d–%d\n" +
		"מחיר מקסימלי: %s ₪\n" +
		"מנוע: %s\n" +
		"ק״מ מקסימלי: %s\n" +
		"יד מקסימלית: %s",
	"wizard_confirm_keywords":      "\nמילות מפתח: %s",
	"wizard_confirm_exclude_keys":  "\nמילות החרגה: %s",
	"wizard_search_saved":          "חיפוש #%d נשמר! בודק %s עכשיו...\n\nהשתמש ב /list כדי לראות את החיפושים שלך.",
	"wizard_search_updated":        "חיפוש #%d עודכן!\n\nהשתמש ב /list כדי לראות את החיפושים שלך.",
	"wizard_save_failed":           "שמירת החיפוש נכשלה. אנא נסה שוב.",
	"wizard_session_expired":       "הסשן פג. השתמש ב /watch כדי להתחיל חיפוש חדש.",

	// /watch
	"watch_limit_reached": "כבר יש לך %d חיפושים פעילים (מקסימום %d). השתמש ב /stop כדי למחוק אחד קודם.",
	"watch_limit_error":   "בדיקת מגבלות נכשלה. אנא נסה שוב.",

	// /list
	"list_header":      "*החיפושים שלך (%d):*\n\n",
	"list_empty":       "אין לך חיפושים פעילים. השתמש ב /watch כדי ליצור אחד.",
	"list_load_error":  "טעינת חיפושים נכשלה. אנא נסה שוב.",
	"list_delete_btn":  "מחק #%d",

	// /stop
	"stop_usage":    "שימוש: /stop <מספר\\_חיפוש>\nהשתמש ב /list כדי לראות את מספרי החיפושים.",
	"stop_invalid":  "מספר חיפוש לא תקין. השתמש ב /list כדי לראות את החיפושים.",
	"stop_failed":   "מחיקת חיפוש נכשלה.",
	"stop_success":  "חיפוש #%d נמחק.",

	// /pause
	"pause_usage":          "שימוש: /pause <מספר\\_חיפוש>\nהשתמש ב /list כדי לראות את מספרי החיפושים.",
	"pause_invalid":        "מספר חיפוש לא תקין. השתמש ב /list כדי לראות את החיפושים.",
	"pause_not_found":      "חיפוש לא נמצא. השתמש ב /list כדי לראות את החיפושים.",
	"pause_already_paused": "חיפוש #%d כבר מושהה.",
	"pause_failed":         "השהיית חיפוש נכשלה.",
	"pause_success":        "חיפוש #%d הושהה. השתמש ב /resume %d כדי לחדש אותו.",

	// /resume
	"resume_usage":          "שימוש: /resume <מספר\\_חיפוש>\nהשתמש ב /list כדי לראות את מספרי החיפושים.",
	"resume_invalid":        "מספר חיפוש לא תקין. השתמש ב /list כדי לראות את החיפושים.",
	"resume_not_found":      "חיפוש לא נמצא. השתמש ב /list כדי לראות את החיפושים.",
	"resume_already_active": "חיפוש #%d כבר פעיל.",
	"resume_failed":         "חידוש חיפוש נכשל.",
	"resume_success":        "חיפוש #%d חודש.",

	// /edit
	"edit_usage":     "שימוש: /edit <מספר\\_חיפוש>\nהשתמש ב /list כדי לראות את מספרי החיפושים.",
	"edit_invalid":   "מספר חיפוש לא תקין. השתמש ב /list כדי לראות את החיפושים.",
	"edit_not_found": "חיפוש לא נמצא. השתמש ב /list כדי לראות את החיפושים.",

	// /cancel
	"cancel": "בוטל. השתמש ב /watch כדי להתחיל חיפוש חדש.",

	// /share
	"share_not_configured": "שיתוף לא מוגדר. שם המשתמש של הבוט חסר.",
	"share_usage":          "שימוש: /share <מספר\\_חיפוש>\nהשתמש ב /list כדי לראות את מספרי החיפושים.",
	"share_invalid":        "מספר חיפוש לא תקין. השתמש ב /list כדי לראות את החיפושים.",
	"share_not_found":      "חיפוש לא נמצא. השתמש ב /list כדי לראות את החיפושים.",
	"share_link":           "שתף את הקישור הזה לחיפוש *%s %s*:\n\n%s",
	"share_invalid_link":   "קישור שיתוף לא תקין.",
	"share_search_deleted": "החיפוש המשותף לא נמצא. ייתכן שנמחק.",
	"share_limit_error":    "בדיקת מגבלות נכשלה. אנא נסה שוב.",
	"share_limit_reached":  "כבר יש לך %d חיפושים פעילים (מקסימום %d). השתמש ב /stop כדי למחוק אחד קודם.",
	"share_copy_failed":    "העתקת חיפוש נכשלה. אנא נסה שוב.",
	"share_copy_success":   "חיפוש #%d נשמר! אבדוק %s כל %s ואשלח לך מודעות חדשות.\n\nהשתמש ב /list כדי לראות את החיפושים שלך.",
	"share_copy_btn":       "העתק חיפוש זה",
	"share_summary": "*חיפוש משותף:*\n" +
		"רכב: %s %s\n" +
		"שנים: %d–%d\n" +
		"מחיר מקסימלי: %s ₪\n" +
		"מנוע: %s\n\n" +
		"להעתיק את החיפוש ולהתחיל לקבל התראות?",

	// /history
	"history_unavailable":     "היסטוריה לא זמינה.",
	"history_load_error":      "טעינת היסטוריה נכשלה. אנא נסה שוב.",
	"history_empty":           "אין התאמות עדיין. השתמש ב /watch כדי להגדיר חיפוש.",
	"history_page_invalid":    "דף ההיסטוריה לא זמין יותר. השתמש ב /history כדי להתחיל מחדש.",
	"history_header":          "*היסטוריית התאמות (%d סה״כ):*\n",
	"history_found":           "📅 נמצא: %s\n",
	"history_newer":           "← חדשים יותר",
	"history_older":           "ישנים יותר →",

	// /digest
	"digest_unavailable":     "מצב סיכום לא זמין.",
	"digest_load_error":      "טעינת הגדרות סיכום נכשלה.",
	"digest_mode_digest":     "*מצב התראות:* סיכום (כל %s)\n\nבחר תדירות או עבור למיידי:",
	"digest_mode_instant":    "*מצב התראות:* מיידי\n\nעבור למצב סיכום — בחר כל כמה זמן לקבל מודעות מרוכזות:",
	"digest_switched_digest": "עברת למצב *סיכום*. מודעות ירוכזו ויישלחו כל %s.",
	"digest_switched_instant":"עברת למצב *מיידי*. מודעות יישלחו מיד.",
	"digest_update_failed":   "עדכון מצב סיכום נכשל.",
	"digest_invalid_interval":"תדירות לא תקינה.",

	// /settings
	"settings": "*ההגדרות שלך:*\nחיפושים פעילים: %d/%d",

	// /language
	"language_current":  "*שפה נוכחית:* עברית\n\nבחר שפה:",
	"language_switched": "השפה שונתה לעברית.",

	// /saved
	"saved_empty":      "אין לך רכבים שמורים עדיין.",
	"saved_header":     "*רכבים שמורים (%d):*\n",
	"saved_load_error": "טעינת שמורים נכשלה. אנא נסה שוב.",

	// /hidden
	"hidden_empty":      "אין לך רכבים מוסתרים.",
	"hidden_header":     "*רכבים מוסתרים (%d):*\n",
	"hidden_clear_btn":  "נקה הכל",
	"hidden_cleared":    "כל הרכבים המוסתרים נוקו.",

	// listing actions
	"listing_saved":  "נשמר!",
	"listing_hidden": "הוסתר",

	// generic
	"error_generic":           "משהו השתבש. אנא נסה שוב.",
	"error_invalid_id":        "מספר חיפוש לא תקין.",
	"error_wrong_state":       "משהו השתבש. השתמש ב /cancel ונסה שוב.",
	"unknown_command":         "לא הבנתי. השתמש ב /help לרשימת הפקודות.",

	// keyboard buttons
	"btn_done":       "סיום ✓",
	"btn_search":     "חיפוש",
	"btn_any_model":  "כל דגם",
	"btn_previous":   "הקודם",
	"btn_next":       "הבא",
	"btn_no_results": "לא נמצאו תוצאות",
	"btn_back":       "חזרה לרשימה",
	"btn_confirm":    "אישור",
	"btn_start_over": "התחל מחדש",
	"btn_cancel":     "ביטול",
	"btn_skip":       "דלג",
	"btn_save":       "שמור",
	"btn_hide":       "הסתר",
	"btn_quick_start":"התחלה מהירה",
	"btn_custom":     "חיפוש מותאם אישית",

	// engine options
	"btn_any_engine": "כל מנוע",

	// km options
	"btn_any": "כלשהו",

	// hand options
	"btn_hand_1": "ראשונה",
	"btn_hand_2": "שנייה",
	"btn_hand_3": "שלישית",
	"btn_hand_4": "רביעית",

	// digest buttons
	"btn_switch_instant": "עבור למיידי",

	// confirm summary labels
	"label_any":      "כלשהו",
	"label_active":   "פעיל",
	"label_paused":   "מושהה",

	// formatter
	"fmt_new_listing":      "🚗 *רכב חדש*\n\n",
	"fmt_year":             "📅 שנה: %d",
	"fmt_year_month":       "/%02d",
	"fmt_engine":           "⚙️ מנוע: %.1fL",
	"fmt_power":            "🐴 כ\"ס: %d\n",
	"fmt_mileage":          "🛣️ ק\"מ: %s\n",
	"fmt_hand":             "✋ יד: %d\n",
	"fmt_location":         "📍 מיקום: %s\n",
	"fmt_price":            "💰 מחיר: ₪%s\n",
	"fmt_price_drop":       "💰 *ירידת מחיר!* %s: ₪%s → ₪%s (-₪%s)\n",
	"fmt_batch_header":     "🚗 *%d מודעות חדשות*\n",
	"fmt_batch_item":       "*[%d/%d]*\n",
	"fmt_digest_header":    "*סיכום יומי (%d פריטים):*\n",

	// deal scoring
	"fmt_deal_score":        "📊 ציון עסקה: %d/100\n",
	"fmt_deal_below_market": "%d%% מתחת לשוק (חציון ₪%s · %d מודעות)\n",
	"fmt_deal_near_market":  "קרוב למחיר השוק (חציון ₪%s · %d מודעות)\n",
	"fmt_deal_above_market": "מעל מחיר השוק (חציון ₪%s · %d מודעות)\n",
	"fmt_deal_no_data":      "📊 _אין מספיק נתוני שוק עדיין_\n",

	// daily market digest
	"fmt_market_digest_header":     "📈 *סיכום שוק יומי* — %s\n\n",
	"fmt_market_digest_search":     "*%s:*\n",
	"fmt_market_digest_new":        "  🆕 חדשות (24ש): %d\n",
	"fmt_market_digest_avg":        "  💰 מחיר ממוצע: ₪%s\n",
	"fmt_market_digest_best":       "  ⭐ הכי טוב: ₪%s\n",
	"fmt_market_digest_best_link":  "    🔗 %s\n",
	"fmt_market_digest_trend_up":   "  📈 מגמה עולה %.1f%%\n",
	"fmt_market_digest_trend_down": "  📉 מגמה יורדת %.1f%%\n",
	"fmt_market_digest_trend_flat": "  ➡️ מחירים יציבים\n",

	// daily digest settings
	"daily_digest_enabled":  "סיכום שוק יומי *מופעל* בשעה %s (שעון ישראל).",
	"daily_digest_disabled": "סיכום שוק יומי *כבוי*.",
	"btn_daily_digest_on":   "📈 הפעל סיכום יומי",
	"btn_daily_digest_off":  "📈 כבה סיכום יומי",

	// onboarding
	"onboarding_welcome": "ברוכים הבאים ל-*CarWatch*! 🚗\n\n" +
		"אני עוקב אחרי מודעות רכב ביד2 ו-WinWin ושולח לך התראות כשמופיעים רכבים חדשים.\n\n" +
		"הנה דוגמה להתראה שתקבל:",
	"onboarding_post_search": "הכל מוכן! אתחיל לבדוק מיד. ציפייה להתראה הראשונה תוך %s.",

	// tier system
	"tier_free":    "חינמי",
	"tier_premium": "פרימיום",
	"tier_trial":   "ניסיון",
	"tier_expires": "פג תוקף ב: %s",

	"settings_tier":         "\nמנוי: %s",
	"settings_tier_trial":   "\nמנוי: %s (ניסיון — פג ב: %s)",
	"settings_tier_premium": "\nמנוי: %s (פג ב: %s)",

	"upgrade_prompt": "תכונה זו זמינה רק למנויי *פרימיום*.\n\n" +
		"שדרג ב-₪29/חודש כדי לקבל:\n" +
		"• עד 10 חיפושים פעילים\n" +
		"• ציוני עסקה לכל מודעה\n" +
		"• סיכום שוק יומי\n" +
		"• התראות על ירידות מחיר\n\n" +
		"השתמש ב /upgrade להוראות שדרוג.",
	"upgrade_info": "*שדרוג לפרימיום — ₪29/חודש*\n\n" +
		"✅ עד 10 חיפושים פעילים\n" +
		"✅ ציוני עסקה לכל מודעה\n" +
		"✅ סיכום שוק יומי\n" +
		"✅ התראות על ירידות מחיר\n\n" +
		"לשדרוג, שלח תשלום דרך Bit / PayBox וצרף אישור לצ'אט.\n" +
		"מנהל יפעיל את המנוי שלך.",
	"upgrade_search_limit": "הגעת למגבלה של %d חיפושים (מקסימום %d לחינמיים).\n\nשדרג לפרימיום כדי ליצור עד 10 חיפושים. השתמש ב /upgrade לפרטים.",

	"trial_welcome":  "🎉 קיבלת 7 ימי ניסיון *פרימיום*! כל התכונות פתוחות עד %s.",
	"trial_expired":  "תקופת הניסיון הסתיימה. חזרת למנוי *חינמי*.\nהשתמש ב /upgrade כדי להמשיך ליהנות מתכונות פרימיום.",
	"premium_expired":"מנוי הפרימיום שלך פג. חזרת למנוי *חינמי*.\nהשתמש ב /upgrade לחידוש.",

	"admin_grant_usage":   "שימוש: /grant\\_premium <chat\\_id> <days>",
	"admin_grant_success": "פרימיום הופעל למשתמש %d עד %s.",
	"admin_grant_failed":  "הפעלת פרימיום נכשלה.",
	"admin_revoke_usage":  "שימוש: /revoke\\_premium <chat\\_id>",
	"admin_revoke_success":"פרימיום בוטל למשתמש %d.",
	"admin_revoke_failed": "ביטול פרימיום נכשל.",
}

var en = map[string]string{
	// /start
	"welcome": "Welcome to *CarWatch*! I monitor car listings on Yad2 and WinWin and send you alerts when new matches appear.\n\n" +
		"Use /watch to set up a new car search.\n" +
		"Use /list to see your active searches.\n" +
		"Use /help for all commands.",

	// /help
	"help": "*CarWatch Commands:*\n\n" +
		"/watch — Set up a new car search\n" +
		"/list — Show your active searches\n" +
		"/edit <id> — Edit an existing search\n" +
		"/history — View past matched listings\n" +
		"/pause <id> — Pause a search\n" +
		"/resume <id> — Resume a paused search\n" +
		"/stop <id> — Delete a search\n" +
		"/share <id> — Share a search via link\n" +
		"/saved — View saved listings\n" +
		"/hidden — View hidden listings\n" +
		"/digest — Notifications & daily market summary\n" +
		"/upgrade — Upgrade to Premium\n" +
		"/language — Change language\n" +
		"/settings — View your settings\n" +
		"/cancel — Cancel current wizard\n" +
		"/help — Show this message",

	// wizard - source
	"wizard_source_prompt":  "Which marketplaces do you want to search? (select one or both)",
	"wizard_source_empty":   "Please select at least one marketplace.",
	"wizard_start_over":     "Let's start over. Which marketplaces?",

	// wizard - manufacturer
	"wizard_mfr_prompt":     "What manufacturer are you looking for?",
	"wizard_mfr_search":     "Type the manufacturer name:",
	"wizard_mfr_results":    "Search results:",

	// wizard - model
	"wizard_model_prompt":   "Which %s model?",
	"wizard_model_search":   "Type the %s model name:",
	"wizard_model_results":  "Search results:",

	// wizard - year
	"wizard_year_min":       "From which year? (e.g. 2018)",
	"wizard_year_max":       "Until which year? (e.g. 2024)",
	"wizard_year_invalid":   "Please enter a valid year (%d–%d).",
	"wizard_year_min_error": "Must be >= %d. Try again.",

	// wizard - price
	"wizard_price_prompt":   "Max price in NIS? (e.g. 150000)",
	"wizard_price_invalid":  "Please enter a valid price (1,000–10,000,000).",

	// wizard - engine
	"wizard_engine_prompt":  "Minimum engine size?",

	// wizard - km
	"wizard_km_prompt":      "Maximum kilometers?",

	// wizard - hand
	"wizard_hand_prompt":    "Maximum ownership hand?",

	// wizard - keywords
	"wizard_keywords_prompt":      "Any keywords to require in the description? (comma-separated, or type 'skip')\nExample: automatic, well-kept",
	"wizard_exclude_keys_prompt":  "Any keywords to exclude? (comma-separated, or type 'skip')\nExample: accident, damaged",
	"wizard_keywords_skip":        "skip",

	// wizard - confirm
	"wizard_confirm_summary": "*Your search:*\n" +
		"Source: %s\n" +
		"Car: %s %s\n" +
		"Year: %d–%d\n" +
		"Max price: %s NIS\n" +
		"Engine: %s\n" +
		"Max km: %s\n" +
		"Max hand: %s",
	"wizard_confirm_keywords":      "\nKeywords: %s",
	"wizard_confirm_exclude_keys":  "\nExclude: %s",
	"wizard_search_saved":          "Search #%d saved! Checking %s now...\n\nUse /list to see your searches.",
	"wizard_search_updated":        "Search #%d updated!\n\nUse /list to see your searches.",
	"wizard_save_failed":           "Failed to save search. Please try again.",
	"wizard_session_expired":       "Session expired. Use /watch to start a new search.",

	// /watch
	"watch_limit_reached": "You already have %d active searches (max %d). Use /stop to remove one first.",
	"watch_limit_error":   "Failed to check search limits. Please try again.",

	// /list
	"list_header":      "*Your searches (%d):*\n\n",
	"list_empty":       "You have no active searches. Use /watch to create one.",
	"list_load_error":  "Failed to load searches. Please try again.",
	"list_delete_btn":  "Delete #%d",

	// /stop
	"stop_usage":    "Usage: /stop <search\\_id>\nUse /list to see your search IDs.",
	"stop_invalid":  "Invalid search ID. Use /list to see your searches.",
	"stop_failed":   "Failed to delete search.",
	"stop_success":  "Search #%d deleted.",

	// /pause
	"pause_usage":          "Usage: /pause <search\\_id>\nUse /list to see your search IDs.",
	"pause_invalid":        "Invalid search ID. Use /list to see your searches.",
	"pause_not_found":      "Search not found. Use /list to see your searches.",
	"pause_already_paused": "Search #%d is already paused.",
	"pause_failed":         "Failed to pause search.",
	"pause_success":        "Search #%d paused. Use /resume %d to resume it.",

	// /resume
	"resume_usage":          "Usage: /resume <search\\_id>\nUse /list to see your search IDs.",
	"resume_invalid":        "Invalid search ID. Use /list to see your searches.",
	"resume_not_found":      "Search not found. Use /list to see your searches.",
	"resume_already_active": "Search #%d is already active.",
	"resume_failed":         "Failed to resume search.",
	"resume_success":        "Search #%d resumed.",

	// /edit
	"edit_usage":     "Usage: /edit <search\\_id>\nUse /list to see your search IDs.",
	"edit_invalid":   "Invalid search ID. Use /list to see your searches.",
	"edit_not_found": "Search not found. Use /list to see your searches.",

	// /cancel
	"cancel": "Cancelled. Use /watch to start a new search.",

	// /share
	"share_not_configured": "Sharing is not configured. Bot username is missing.",
	"share_usage":          "Usage: /share <search\\_id>\nUse /list to see your search IDs.",
	"share_invalid":        "Invalid search ID. Use /list to see your searches.",
	"share_not_found":      "Search not found. Use /list to see your searches.",
	"share_link":           "Share this link for *%s %s* search:\n\n%s",
	"share_invalid_link":   "Invalid share link.",
	"share_search_deleted": "The shared search was not found. It may have been deleted.",
	"share_limit_error":    "Failed to check search limits. Please try again.",
	"share_limit_reached":  "You already have %d active searches (max %d). Use /stop to remove one first.",
	"share_copy_failed":    "Failed to copy search. Please try again.",
	"share_copy_success":   "Search #%d saved! I'll check %s every %s and send you new listings.\n\nUse /list to see your searches.",
	"share_copy_btn":       "Copy this search",
	"share_summary": "*Shared search:*\n" +
		"Car: %s %s\n" +
		"Year: %d–%d\n" +
		"Max price: %s NIS\n" +
		"Engine: %s\n\n" +
		"Copy this search to start receiving alerts?",

	// /history
	"history_unavailable":     "History is not available.",
	"history_load_error":      "Failed to load history. Please try again.",
	"history_empty":           "No matched listings yet. Use /watch to set up a search.",
	"history_page_invalid":    "That history page is no longer available. Use /history to start again.",
	"history_header":          "*Match history (%d total):*\n",
	"history_found":           "📅 Found: %s\n",
	"history_newer":           "← Newer",
	"history_older":           "Older →",

	// /digest
	"digest_unavailable":     "Digest mode is not available.",
	"digest_load_error":      "Failed to load digest settings.",
	"digest_mode_digest":     "*Notification mode:* digest (every %s)\n\nChoose interval or switch to instant:",
	"digest_mode_instant":    "*Notification mode:* instant\n\nSwitch to digest mode — choose how often to receive batched listings:",
	"digest_switched_digest": "Switched to *digest* mode. Listings will be batched and sent every %s.",
	"digest_switched_instant":"Switched to *instant* mode. Listings will be sent immediately.",
	"digest_update_failed":   "Failed to update digest mode.",
	"digest_invalid_interval":"Invalid interval.",

	// /settings
	"settings": "*Your settings:*\nActive searches: %d/%d",

	// /language
	"language_current":  "*Current language:* English\n\nChoose language:",
	"language_switched": "Language changed to English.",

	// /saved
	"saved_empty":      "You have no saved listings yet.",
	"saved_header":     "*Saved listings (%d):*\n",
	"saved_load_error": "Failed to load saved listings. Please try again.",

	// /hidden
	"hidden_empty":      "You have no hidden listings.",
	"hidden_header":     "*Hidden listings (%d):*\n",
	"hidden_clear_btn":  "Clear all",
	"hidden_cleared":    "All hidden listings cleared.",

	// listing actions
	"listing_saved":  "Saved!",
	"listing_hidden": "Hidden",

	// generic
	"error_generic":           "Something went wrong. Please try again.",
	"error_invalid_id":        "Invalid search ID.",
	"error_wrong_state":       "Something went wrong. Use /cancel and try again.",
	"unknown_command":         "I didn't understand that. Use /help for available commands.",

	// keyboard buttons
	"btn_done":       "Done ✓",
	"btn_search":     "Search",
	"btn_any_model":  "Any model",
	"btn_previous":   "Previous",
	"btn_next":       "Next",
	"btn_no_results": "No results found",
	"btn_back":       "Back to list",
	"btn_confirm":    "Confirm",
	"btn_start_over": "Start over",
	"btn_cancel":     "Cancel",
	"btn_skip":       "Skip",
	"btn_save":       "Save",
	"btn_hide":       "Hide",
	"btn_quick_start":"Quick Start",
	"btn_custom":     "Custom Search",

	// engine options
	"btn_any_engine": "Any engine",

	// km options
	"btn_any": "Any",

	// hand options
	"btn_hand_1": "1st",
	"btn_hand_2": "2nd",
	"btn_hand_3": "3rd",
	"btn_hand_4": "4th",

	// digest buttons
	"btn_switch_instant": "Switch to instant",

	// confirm summary labels
	"label_any":      "Any",
	"label_active":   "active",
	"label_paused":   "paused",

	// formatter
	"fmt_new_listing":      "🚗 *New Car Listing*\n\n",
	"fmt_year":             "📅 Year: %d",
	"fmt_year_month":       "/%02d",
	"fmt_engine":           "⚙️ Engine: %.1fL",
	"fmt_power":            "🐴 Power: %d HP\n",
	"fmt_mileage":          "🛣️ Mileage: %s km\n",
	"fmt_hand":             "✋ Hand: %d\n",
	"fmt_location":         "📍 Location: %s\n",
	"fmt_price":            "💰 Price: ₪%s\n",
	"fmt_price_drop":       "💰 *Price Drop!* %s: ₪%s → ₪%s (-₪%s)\n",
	"fmt_batch_header":     "🚗 *%d New Listings Found*\n",
	"fmt_batch_item":       "*[%d/%d]*\n",
	"fmt_digest_header":    "*Digest Summary (%d items):*\n",

	// deal scoring
	"fmt_deal_score":        "📊 Deal Score: %d/100\n",
	"fmt_deal_below_market": "%d%% below market (₪%s median · %d listings)\n",
	"fmt_deal_near_market":  "Near market price (₪%s median · %d listings)\n",
	"fmt_deal_above_market": "Above market price (₪%s median · %d listings)\n",
	"fmt_deal_no_data":      "📊 _Not enough market data yet_\n",

	// daily market digest
	"fmt_market_digest_header":     "📈 *Daily Market Summary* — %s\n\n",
	"fmt_market_digest_search":     "*%s:*\n",
	"fmt_market_digest_new":        "  🆕 New (24h): %d\n",
	"fmt_market_digest_avg":        "  💰 Avg price: ₪%s\n",
	"fmt_market_digest_best":       "  ⭐ Best: ₪%s\n",
	"fmt_market_digest_best_link":  "    🔗 %s\n",
	"fmt_market_digest_trend_up":   "  📈 Trending up %.1f%%\n",
	"fmt_market_digest_trend_down": "  📉 Trending down %.1f%%\n",
	"fmt_market_digest_trend_flat": "  ➡️ Prices stable\n",

	// daily digest settings
	"daily_digest_enabled":  "Daily market summary *enabled* at %s Israel time.",
	"daily_digest_disabled": "Daily market summary *disabled*.",
	"btn_daily_digest_on":   "📈 Enable Daily Summary",
	"btn_daily_digest_off":  "📈 Disable Daily Summary",

	// onboarding
	"onboarding_welcome": "Welcome to *CarWatch*! 🚗\n\n" +
		"I monitor car listings on Yad2 and WinWin and send you alerts when new cars match your criteria.\n\n" +
		"Here's an example of the alerts you'll receive:",
	"onboarding_post_search": "You're all set! I'll start checking right away. Expect your first alert within %s.",

	// tier system
	"tier_free":    "Free",
	"tier_premium": "Premium",
	"tier_trial":   "Trial",
	"tier_expires": "Expires: %s",

	"settings_tier":         "\nPlan: %s",
	"settings_tier_trial":   "\nPlan: %s (trial — expires %s)",
	"settings_tier_premium": "\nPlan: %s (expires %s)",

	"upgrade_prompt": "This feature is available to *Premium* subscribers only.\n\n" +
		"Upgrade for ₪29/month to get:\n" +
		"• Up to 10 active searches\n" +
		"• Deal scores on every listing\n" +
		"• Daily market summary\n" +
		"• Price drop alerts\n\n" +
		"Use /upgrade for upgrade instructions.",
	"upgrade_info": "*Upgrade to Premium — ₪29/month*\n\n" +
		"✅ Up to 10 active searches\n" +
		"✅ Deal scores on every listing\n" +
		"✅ Daily market summary\n" +
		"✅ Price drop alerts\n\n" +
		"To upgrade, send payment via Bit / PayBox and forward the confirmation here.\n" +
		"An admin will activate your subscription.",
	"upgrade_search_limit": "You've reached the limit of %d searches (max %d for Free plan).\n\nUpgrade to Premium for up to 10 searches. Use /upgrade for details.",

	"trial_welcome":  "🎉 You've received a 7-day *Premium* trial! All features unlocked until %s.",
	"trial_expired":  "Your trial has ended. You're back on the *Free* plan.\nUse /upgrade to keep enjoying Premium features.",
	"premium_expired":"Your Premium subscription has expired. You're back on the *Free* plan.\nUse /upgrade to renew.",

	"admin_grant_usage":   "Usage: /grant\\_premium <chat\\_id> <days>",
	"admin_grant_success": "Premium activated for user %d until %s.",
	"admin_grant_failed":  "Failed to activate premium.",
	"admin_revoke_usage":  "Usage: /revoke\\_premium <chat\\_id>",
	"admin_revoke_success":"Premium revoked for user %d.",
	"admin_revoke_failed": "Failed to revoke premium.",
}

package locale

var heScoring = map[string]string{
	"fmt_fitness_score":     "🎯 התאמה: %.1f/10\n",
	"fmt_fitness_up_down":   "   ↑ %s  ↓ %s\n",
	"fmt_fitness_up_only":   "   ↑ %s\n",
	"fmt_fitness_down_only": "   ↓ %s\n",
	"fmt_deal_score":        "📊 ציון עסקה: %d/100\n",
	"fmt_deal_below_market": "%d%% מתחת לשוק (חציון ₪%s · %d מודעות)\n",
	"fmt_deal_near_market":  "קרוב למחיר השוק (חציון ₪%s · %d מודעות)\n",
	"fmt_deal_above_market": "מעל מחיר השוק (חציון ₪%s · %d מודעות)\n",
	"fmt_deal_no_data":      "📊 _אין מספיק נתוני שוק עדיין_\n",

	// daily market digest
}

var enScoring = map[string]string{
	"fmt_fitness_score":     "🎯 Fitness: %.1f/10\n",
	"fmt_fitness_up_down":   "   ↑ %s  ↓ %s\n",
	"fmt_fitness_up_only":   "   ↑ %s\n",
	"fmt_fitness_down_only": "   ↓ %s\n",
	"fmt_deal_score":        "📊 Deal Score: %d/100\n",
	"fmt_deal_below_market": "%d%% below market (₪%s median · %d listings)\n",
	"fmt_deal_near_market":  "Near market price (₪%s median · %d listings)\n",
	"fmt_deal_above_market": "Above market price (₪%s median · %d listings)\n",
	"fmt_deal_no_data":      "📊 _Not enough market data yet_\n",

	// daily market digest
}

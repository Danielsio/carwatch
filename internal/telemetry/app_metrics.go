package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter = otel.Meter("carwatch")

	// ScrapesDuration records how long each source scrape takes.
	ScrapesDuration metric.Float64Histogram
	// ListingsFetched counts total listings returned from sources.
	ListingsFetched metric.Int64Counter
	// ListingsMatched counts new listings that matched a user search.
	ListingsMatched metric.Int64Counter
	// NotificationsSent counts notifications delivered to users.
	NotificationsSent metric.Int64Counter
	// SchedulerCycles counts completed scheduler polling cycles.
	SchedulerCycles metric.Int64Counter
	// ActiveSearches reports the current number of active searches.
	ActiveSearches metric.Int64Gauge
	// ActiveUsers reports the current number of active users.
	ActiveUsers metric.Int64Gauge

	// EnrichRequests counts enrichment requests published to the stream.
	EnrichRequests metric.Int64Counter
	// EnrichSuccesses counts successful enrichments.
	EnrichSuccesses metric.Int64Counter
	// EnrichChallenges counts bot challenge responses during enrichment.
	EnrichChallenges metric.Int64Counter
	// EnrichSkipped counts enrichment requests skipped (already enriched).
	EnrichSkipped metric.Int64Counter
)

// InitMetrics creates all application-level OTel metric instruments.
// Call this once after the MeterProvider has been installed.
func InitMetrics() error {
	var err error

	ScrapesDuration, err = meter.Float64Histogram("carwatch.scrape.duration",
		metric.WithDescription("Time to scrape a source"),
		metric.WithUnit("s"))
	if err != nil {
		return err
	}

	ListingsFetched, err = meter.Int64Counter("carwatch.listings.fetched",
		metric.WithDescription("Total listings fetched from sources"))
	if err != nil {
		return err
	}

	ListingsMatched, err = meter.Int64Counter("carwatch.listings.matched",
		metric.WithDescription("New listings matched to searches"))
	if err != nil {
		return err
	}

	NotificationsSent, err = meter.Int64Counter("carwatch.notifications.sent",
		metric.WithDescription("Notifications sent"))
	if err != nil {
		return err
	}

	SchedulerCycles, err = meter.Int64Counter("carwatch.scheduler.cycles",
		metric.WithDescription("Scheduler polling cycles"))
	if err != nil {
		return err
	}

	ActiveSearches, err = meter.Int64Gauge("carwatch.searches.active",
		metric.WithDescription("Number of active searches"))
	if err != nil {
		return err
	}

	ActiveUsers, err = meter.Int64Gauge("carwatch.users.active",
		metric.WithDescription("Number of active users"))
	if err != nil {
		return err
	}

	EnrichRequests, err = meter.Int64Counter("carwatch.enrich.requests",
		metric.WithDescription("Enrichment requests published to stream"))
	if err != nil {
		return err
	}

	EnrichSuccesses, err = meter.Int64Counter("carwatch.enrich.successes",
		metric.WithDescription("Successful enrichments"))
	if err != nil {
		return err
	}

	EnrichChallenges, err = meter.Int64Counter("carwatch.enrich.challenges",
		metric.WithDescription("Bot challenge responses during enrichment"))
	if err != nil {
		return err
	}

	EnrichSkipped, err = meter.Int64Counter("carwatch.enrich.skipped",
		metric.WithDescription("Enrichment requests skipped (already enriched)"))
	if err != nil {
		return err
	}

	return nil
}

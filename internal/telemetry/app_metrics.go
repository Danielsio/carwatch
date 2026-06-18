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
	// EnrichItemsGone counts listings dropped because their source page is gone
	// (HTTP 404/410) and they no longer exist at the source.
	EnrichItemsGone metric.Int64Counter

	// PersistFailures counts failed listing-batch persists (retried next cycle).
	PersistFailures metric.Int64Counter
	// ClaimReleaseFailures counts failed dedup-claim releases after a persist or
	// delivery failure — the only permanent listing-loss path.
	ClaimReleaseFailures metric.Int64Counter

	// QueueDepth reports the current number of messages in the alerts stream.
	QueueDepth metric.Int64Gauge
	// QueuePending reports the number of messages claimed but not yet acked.
	QueuePending metric.Int64Gauge
	// QueueLag records the time between message publish and delivery.
	QueueLag metric.Float64Histogram

	// VitalsCLS records Cumulative Layout Shift values from the browser.
	VitalsCLS metric.Float64Histogram
	// VitalsFCP records First Contentful Paint values from the browser.
	VitalsFCP metric.Float64Histogram
	// VitalsINP records Interaction to Next Paint values from the browser.
	VitalsINP metric.Float64Histogram
	// VitalsLCP records Largest Contentful Paint values from the browser.
	VitalsLCP metric.Float64Histogram
	// VitalsTTFB records Time to First Byte values from the browser.
	VitalsTTFB metric.Float64Histogram
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

	EnrichItemsGone, err = meter.Int64Counter("carwatch.enrich.items_gone",
		metric.WithDescription("Listings dropped because their source page is gone (404/410)"))
	if err != nil {
		return err
	}

	PersistFailures, err = meter.Int64Counter("carwatch.listings.persist_failures",
		metric.WithDescription("Failed listing-batch persists (retried next cycle)"))
	if err != nil {
		return err
	}

	ClaimReleaseFailures, err = meter.Int64Counter("carwatch.dedup.claim_release_failures",
		metric.WithDescription("Failed dedup-claim releases (permanent listing loss path)"))
	if err != nil {
		return err
	}

	QueueDepth, err = meter.Int64Gauge("carwatch.queue.depth",
		metric.WithDescription("Messages in alerts stream"))
	if err != nil {
		return err
	}

	QueuePending, err = meter.Int64Gauge("carwatch.queue.pending",
		metric.WithDescription("Messages claimed but not yet acked"))
	if err != nil {
		return err
	}

	QueueLag, err = meter.Float64Histogram("carwatch.queue.lag",
		metric.WithDescription("Time from message publish to delivery"),
		metric.WithUnit("s"))
	if err != nil {
		return err
	}

	VitalsCLS, err = meter.Float64Histogram("carwatch.vitals.cls",
		metric.WithDescription("Cumulative Layout Shift from browser"))
	if err != nil {
		return err
	}

	VitalsFCP, err = meter.Float64Histogram("carwatch.vitals.fcp",
		metric.WithDescription("First Contentful Paint"),
		metric.WithUnit("ms"))
	if err != nil {
		return err
	}

	VitalsINP, err = meter.Float64Histogram("carwatch.vitals.inp",
		metric.WithDescription("Interaction to Next Paint"),
		metric.WithUnit("ms"))
	if err != nil {
		return err
	}

	VitalsLCP, err = meter.Float64Histogram("carwatch.vitals.lcp",
		metric.WithDescription("Largest Contentful Paint"),
		metric.WithUnit("ms"))
	if err != nil {
		return err
	}

	VitalsTTFB, err = meter.Float64Histogram("carwatch.vitals.ttfb",
		metric.WithDescription("Time to First Byte"),
		metric.WithUnit("ms"))
	if err != nil {
		return err
	}

	return nil
}

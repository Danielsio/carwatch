package scheduler

import "time"

type CycleObserver interface {
	RecordSuccess()
	RecordError()
	RecordListingsFound(n int)
	RecordNotificationSent()
	RecordFetch(source string, duration time.Duration, err error)
	// RecordPersistFailure counts a failed listing-batch persist (listings are
	// retried next cycle via released dedup claims).
	RecordPersistFailure()
	// RecordClaimReleaseFailure counts a failed dedup-claim release after a
	// persist/delivery failure — the one path where a listing can be
	// permanently lost for a user.
	RecordClaimReleaseFailure()
}

type nopObserver struct{}

func (nopObserver) RecordSuccess()                           {}
func (nopObserver) RecordError()                             {}
func (nopObserver) RecordListingsFound(int)                  {}
func (nopObserver) RecordNotificationSent()                  {}
func (nopObserver) RecordFetch(string, time.Duration, error) {}
func (nopObserver) RecordPersistFailure()                    {}
func (nopObserver) RecordClaimReleaseFailure()               {}

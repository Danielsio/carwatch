package scheduler

import "time"

type CycleObserver interface {
	RecordSuccess()
	RecordError()
	RecordListingsFound(n int)
	RecordNotificationSent()
	RecordFetch(source string, duration time.Duration, err error)
}

type nopObserver struct{}

func (nopObserver) RecordSuccess()                                     {}
func (nopObserver) RecordError()                                       {}
func (nopObserver) RecordListingsFound(int)                            {}
func (nopObserver) RecordNotificationSent()                            {}
func (nopObserver) RecordFetch(string, time.Duration, error)           {}

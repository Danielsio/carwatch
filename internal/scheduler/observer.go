package scheduler

type CycleObserver interface {
	RecordSuccess()
	RecordError()
	RecordListingsFound(n int)
	RecordNotificationSent()
}

type nopObserver struct{}

func (nopObserver) RecordSuccess()          {}
func (nopObserver) RecordError()            {}
func (nopObserver) RecordListingsFound(int) {}
func (nopObserver) RecordNotificationSent() {}

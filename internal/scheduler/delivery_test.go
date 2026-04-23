package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/model"
)

func TestInstantDelivery_DeliverBatch_Success(t *testing.T) {
	n := &mockNotifier{}
	d := NewInstantDelivery(n, nil)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.messages) != 1 {
		t.Errorf("expected 1 notification, got %d", len(n.messages))
	}
}

func TestInstantDelivery_DeliverBatch_FallsBackToQueue(t *testing.T) {
	n := &mockNotifier{err: errors.New("telegram down")}
	q := &mockNotificationQueue{}
	d := NewInstantDelivery(n, q)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a"}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err != nil {
		t.Errorf("should succeed with queue fallback, got: %v", err)
	}
}

type failQueue struct {
	mockNotificationQueue
	enqueueErr error
}

func (q *failQueue) EnqueueNotification(_ context.Context, _, _, _ string) error {
	return q.enqueueErr
}

type failDigestStore struct {
	mockDigestStore
	addErr error
}

func newFailDigestStore(err error) *failDigestStore {
	return &failDigestStore{
		mockDigestStore: mockDigestStore{
			modes:   make(map[int64]struct{ mode, interval string }),
			items:   make(map[int64][]string),
			flushed: make(map[int64]time.Time),
		},
		addErr: err,
	}
}

func (m *failDigestStore) AddDigestItem(_ context.Context, _ int64, _ string) error {
	return m.addErr
}

func TestInstantDelivery_DeliverBatch_BothFail(t *testing.T) {
	n := &mockNotifier{err: errors.New("telegram down")}
	q := &failQueue{enqueueErr: errors.New("queue full")}
	d := NewInstantDelivery(n, q)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a"}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err == nil {
		t.Fatal("expected error when both notifier and queue fail")
	}
}

func TestInstantDelivery_DeliverBatch_NoQueue(t *testing.T) {
	n := &mockNotifier{err: errors.New("telegram down")}
	d := NewInstantDelivery(n, nil)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a"}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err == nil {
		t.Fatal("expected error when notifier fails and no queue")
	}
}

func TestInstantDelivery_DeliverRaw_Success(t *testing.T) {
	n := &mockNotifier{}
	d := NewInstantDelivery(n, nil)

	err := d.DeliverRaw(context.Background(), 100, "price drop!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.rawMessages) != 1 {
		t.Errorf("expected 1 raw message, got %d", len(n.rawMessages))
	}
	if n.rawMessages[0].recipient != "100" {
		t.Errorf("recipient = %q, want '100'", n.rawMessages[0].recipient)
	}
}

type errRawNotifier struct {
	mockNotifier
	rawErr error
}

func (m *errRawNotifier) NotifyRaw(_ context.Context, recipient string, message string) error {
	if m.rawErr != nil {
		return m.rawErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rawMessages = append(m.rawMessages, rawNotifyCall{recipient: recipient, message: message})
	return nil
}

func TestInstantDelivery_DeliverRaw_FallsBackToQueue(t *testing.T) {
	n := &errRawNotifier{rawErr: errors.New("telegram down")}
	q := &mockNotificationQueue{}
	d := NewInstantDelivery(n, q)

	err := d.DeliverRaw(context.Background(), 100, "price drop!")
	if err != nil {
		t.Errorf("should succeed with queue fallback, got: %v", err)
	}
}

func TestInstantDelivery_DeliverRaw_BothFail(t *testing.T) {
	n := &errRawNotifier{rawErr: errors.New("telegram down")}
	q := &failQueue{enqueueErr: errors.New("queue full")}
	d := NewInstantDelivery(n, q)

	err := d.DeliverRaw(context.Background(), 100, "price drop!")
	if err == nil {
		t.Fatal("expected error when both notifier and queue fail")
	}
}

func TestDigestDelivery_DeliverBatch(t *testing.T) {
	ds := newMockDigestStore()
	d := NewDigestDelivery(ds)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a", Manufacturer: "Toyota", Model: "Corolla", Year: 2021, Price: 100000}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()
	if len(ds.items[100]) != 1 {
		t.Errorf("expected 1 digest item, got %d", len(ds.items[100]))
	}
}

func TestDigestDelivery_DeliverRaw(t *testing.T) {
	ds := newMockDigestStore()
	d := NewDigestDelivery(ds)

	err := d.DeliverRaw(context.Background(), 100, "price drop!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()
	if len(ds.items[100]) != 1 {
		t.Errorf("expected 1 digest item, got %d", len(ds.items[100]))
	}
	if ds.items[100][0] != "price drop!" {
		t.Errorf("item = %q, want 'price drop!'", ds.items[100][0])
	}
}

func TestInstantDelivery_DeliverBatch_QueueOnCancelledCtx(t *testing.T) {
	n := &mockNotifier{err: context.Canceled}
	q := &mockNotificationQueue{}
	d := NewInstantDelivery(n, q)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a"}},
	}

	err := d.DeliverBatch(ctx, 100, listings)
	if err != nil {
		t.Errorf("should enqueue even with cancelled ctx, got: %v", err)
	}
}

func TestInstantDelivery_DeliverRaw_QueueOnCancelledCtx(t *testing.T) {
	n := &errRawNotifier{rawErr: context.Canceled}
	q := &mockNotificationQueue{}
	d := NewInstantDelivery(n, q)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := d.DeliverRaw(ctx, 100, "price drop!")
	if err != nil {
		t.Errorf("should enqueue even with cancelled ctx, got: %v", err)
	}
}

func TestDigestDelivery_DeliverBatch_Error(t *testing.T) {
	ds := newFailDigestStore(errors.New("write failed"))
	d := NewDigestDelivery(ds)

	listings := []model.Listing{
		{RawListing: model.RawListing{Token: "a"}},
	}

	err := d.DeliverBatch(context.Background(), 100, listings)
	if err == nil {
		t.Fatal("expected error")
	}
}

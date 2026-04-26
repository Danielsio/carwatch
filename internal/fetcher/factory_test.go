package fetcher

import (
	"context"
	"testing"

	"github.com/dsionov/carwatch/internal/model"
)

type stubFetcher struct {
	name string
}

func (s *stubFetcher) Fetch(_ context.Context, _ model.SourceParams) ([]model.RawListing, error) {
	return nil, nil
}

func TestFactory_RegisterAndGet(t *testing.T) {
	f := NewFactory()
	yad2 := &stubFetcher{name: "yad2"}
	winwin := &stubFetcher{name: "winwin"}

	f.Register("yad2", yad2)
	f.Register("winwin", winwin)

	got, ok := f.Get("yad2")
	if !ok {
		t.Fatal("expected yad2 fetcher to exist")
	}
	if got.(*stubFetcher).name != "yad2" {
		t.Errorf("got wrong fetcher: %s", got.(*stubFetcher).name)
	}

	got, ok = f.Get("winwin")
	if !ok {
		t.Fatal("expected winwin fetcher to exist")
	}
	if got.(*stubFetcher).name != "winwin" {
		t.Errorf("got wrong fetcher: %s", got.(*stubFetcher).name)
	}
}

func TestFactory_GetUnknown(t *testing.T) {
	f := NewFactory()
	_, ok := f.Get("unknown")
	if ok {
		t.Error("expected false for unknown source")
	}
}

func TestFactory_OverwriteRegistration(t *testing.T) {
	f := NewFactory()
	f.Register("yad2", &stubFetcher{name: "v1"})
	f.Register("yad2", &stubFetcher{name: "v2"})

	got, ok := f.Get("yad2")
	if !ok {
		t.Fatal("expected yad2 fetcher to exist")
	}
	if got.(*stubFetcher).name != "v2" {
		t.Errorf("expected overwritten fetcher, got %s", got.(*stubFetcher).name)
	}
}

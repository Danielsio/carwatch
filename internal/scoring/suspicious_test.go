package scoring

import (
	"testing"

	"github.com/dsionov/carwatch/internal/model"
)

func TestDetectSuspicious_NoMedianPrice(t *testing.T) {
	listing := model.RawListing{Price: 30000}
	reasons := DetectSuspicious(listing, 0)
	if len(reasons) != 0 {
		t.Errorf("expected no reasons with zero median, got %v", reasons)
	}
}

func TestDetectSuspicious_ZeroListingPrice(t *testing.T) {
	reasons := DetectSuspicious(model.RawListing{Price: 0}, 100000)
	if len(reasons) != 0 {
		t.Errorf("expected no reasons with zero listing price, got %v", reasons)
	}
}

func TestDetectSuspicious_PriceBelowMarket(t *testing.T) {
	listing := model.RawListing{Price: 40000, ImageURL: "http://example.com/img.jpg"}
	reasons := DetectSuspicious(listing, 100000)
	if len(reasons) != 1 || reasons[0] != "price_below_market" {
		t.Errorf("expected [price_below_market], got %v", reasons)
	}
}

func TestDetectSuspicious_NoPhotoLowPrice(t *testing.T) {
	// 65000 < 100000*70/100 = 70000, but 65000 >= 100000/2 = 50000
	listing := model.RawListing{Price: 65000, ImageURL: ""}
	reasons := DetectSuspicious(listing, 100000)
	if len(reasons) != 1 || reasons[0] != "no_photo_low_price" {
		t.Errorf("expected [no_photo_low_price], got %v", reasons)
	}
}

func TestDetectSuspicious_BothReasons(t *testing.T) {
	// 30000 < 50000 (median/2) AND no photo + 30000 < 70000 (median*70/100)
	listing := model.RawListing{Price: 30000, ImageURL: ""}
	reasons := DetectSuspicious(listing, 100000)
	if len(reasons) != 2 {
		t.Fatalf("expected 2 reasons, got %v", reasons)
	}
	if reasons[0] != "price_below_market" {
		t.Errorf("first reason should be price_below_market, got %s", reasons[0])
	}
	if reasons[1] != "no_photo_low_price" {
		t.Errorf("second reason should be no_photo_low_price, got %s", reasons[1])
	}
}

func TestDetectSuspicious_NormalPrice(t *testing.T) {
	listing := model.RawListing{Price: 90000, ImageURL: "http://example.com/img.jpg"}
	reasons := DetectSuspicious(listing, 100000)
	if len(reasons) != 0 {
		t.Errorf("expected no reasons for normal listing, got %v", reasons)
	}
}

func TestDetectSuspicious_ExactlyAtHalfMedian(t *testing.T) {
	// Price == median/2 is NOT below market (strictly less than)
	listing := model.RawListing{Price: 50000, ImageURL: "http://example.com/img.jpg"}
	reasons := DetectSuspicious(listing, 100000)
	if len(reasons) != 0 {
		t.Errorf("price at exactly half median should not trigger, got %v", reasons)
	}
}

func TestDetectSuspicious_NoPhotoNormalPrice(t *testing.T) {
	// No photo but price is at 80% of median (not below 70%)
	listing := model.RawListing{Price: 80000, ImageURL: ""}
	reasons := DetectSuspicious(listing, 100000)
	if len(reasons) != 0 {
		t.Errorf("no photo but normal price should not trigger, got %v", reasons)
	}
}

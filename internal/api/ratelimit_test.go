package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestRateLimiter_AllowBurst(t *testing.T) {
	t.Parallel()
	const burst = 5
	rl := newRateLimiter(burst, time.Hour)
	t.Cleanup(rl.stop)

	for i := 0; i < burst; i++ {
		if !rl.allow(42) {
			t.Fatalf("request %d: expected allow=true", i+1)
		}
	}
	if rl.allow(42) {
		t.Fatal("expected allow=false after burst exhausted")
	}
}

func TestRateLimiter_ExhaustedRecovery(t *testing.T) {
	t.Parallel()
	const burst = 2
	every := 25 * time.Millisecond
	rl := newRateLimiter(burst, every)
	t.Cleanup(rl.stop)

	if !rl.allow(7) {
		t.Fatal("expected first token to be allowed")
	}
	if !rl.allow(7) {
		t.Fatal("expected second token to be allowed")
	}
	if rl.allow(7) {
		t.Fatal("expected burst exhausted")
	}

	time.Sleep(every + 10*time.Millisecond)

	if !rl.allow(7) {
		t.Fatal("expected refill after interval elapsed")
	}
}

func TestRateLimiter_MultipleUsers(t *testing.T) {
	t.Parallel()
	const burst = 3
	rl := newRateLimiter(burst, time.Hour)
	t.Cleanup(rl.stop)

	const u1, u2 int64 = 100, 200

	for i := 0; i < burst; i++ {
		if !rl.allow(u1) {
			t.Fatalf("user1 request %d: expected allow=true", i+1)
		}
	}
	if rl.allow(u1) {
		t.Fatal("user1 should be rate limited")
	}

	for i := 0; i < burst; i++ {
		if !rl.allow(u2) {
			t.Fatalf("user2 request %d: expected allow=true", i+1)
		}
	}
	if rl.allow(u2) {
		t.Fatal("user2 should be rate limited independently")
	}
}

func TestIPRateLimiter_AllowBurst(t *testing.T) {
	t.Parallel()
	const burst = 5
	ip := "192.0.2.1"
	rl := newIPRateLimiter(burst, time.Hour, false)
	t.Cleanup(rl.stop)

	for i := 0; i < burst; i++ {
		if !rl.allow(ip) {
			t.Fatalf("request %d: expected allow=true", i+1)
		}
	}
	if rl.allow(ip) {
		t.Fatal("expected allow=false after burst exhausted")
	}
}

func TestIPRateLimiter_BucketFull_RejectsUnknown(t *testing.T) {
	t.Parallel()
	const burst = 5
	rl := newIPRateLimiter(burst, time.Hour, false)
	t.Cleanup(rl.stop)

	for i := range maxIPBuckets {
		ip := fmt.Sprintf("10.%d.%d", i/256, i%256)
		if !rl.allow(ip) {
			t.Fatalf("seed IP %s (%d): expected allow=true", ip, i)
		}
	}

	if rl.allow("192.0.2.99") {
		t.Fatal("expected new unknown IP to be rejected when bucket is full")
	}
}

func TestIPRateLimiter_BucketFull_AllowsExisting(t *testing.T) {
	t.Parallel()
	const burst = 5
	rl := newIPRateLimiter(burst, time.Hour, false)
	t.Cleanup(rl.stop)

	existing := "10.0.0.1"
	for i := range maxIPBuckets {
		ip := existing
		if i > 0 {
			ip = fmt.Sprintf("10.%d.%d", i/256, i%256)
		}
		if !rl.allow(ip) {
			t.Fatalf("seed IP %s (%d): expected allow=true", ip, i)
		}
	}

	if !rl.allow(existing) {
		t.Fatal("expected existing IP to still be allowed when map is full")
	}
}

func TestExtractIP_RemoteAddr(t *testing.T) {
	t.Parallel()
	r := &http.Request{RemoteAddr: "203.0.113.5:12345"}
	got := extractIP(r, false)
	if got != "203.0.113.5" {
		t.Fatalf("extractIP: got %q want %q", got, "203.0.113.5")
	}
}

func TestExtractIP_XForwardedFor_RightmostPublic(t *testing.T) {
	t.Parallel()
	r := &http.Request{
		RemoteAddr: "172.17.0.1:9999",
		Header:     http.Header{"X-Forwarded-For": {"203.0.113.5, 10.0.0.2"}},
	}
	got := extractIP(r, true)
	// rightmost non-private IP is 203.0.113.5 (10.0.0.2 is private)
	if got != "203.0.113.5" {
		t.Fatalf("extractIP: got %q want %q", got, "203.0.113.5")
	}
}

func TestExtractIP_XForwardedFor_SpoofedLeftmost(t *testing.T) {
	t.Parallel()
	// Attacker injects a fake IP as the leftmost entry.
	// The real client IP (added by the trusted proxy) is the rightmost public IP.
	r := &http.Request{
		RemoteAddr: "10.0.0.1:9999",
		Header:     http.Header{"X-Forwarded-For": {"1.2.3.4, 203.0.113.99, 10.0.0.2"}},
	}
	got := extractIP(r, true)
	// Should return 203.0.113.99 (rightmost non-private), not 1.2.3.4 (spoofed)
	if got != "203.0.113.99" {
		t.Fatalf("extractIP: got %q want %q (should use rightmost non-private)", got, "203.0.113.99")
	}
}

func TestExtractIP_XForwardedFor_AllPrivate(t *testing.T) {
	t.Parallel()
	r := &http.Request{
		RemoteAddr: "172.17.0.1:9999",
		Header:     http.Header{"X-Forwarded-For": {"10.0.0.1, 192.168.1.1"}},
	}
	got := extractIP(r, true)
	// All XFF IPs are private, fall back to RemoteAddr
	if got != "172.17.0.1" {
		t.Fatalf("extractIP: got %q want %q (should fall back to RemoteAddr)", got, "172.17.0.1")
	}
}

func TestExtractIP_XForwardedForUntrustedSource(t *testing.T) {
	t.Parallel()
	r := &http.Request{
		RemoteAddr: "198.51.100.2:9999",
		Header:     http.Header{"X-Forwarded-For": {"10.0.0.1"}},
	}
	got := extractIP(r, true)
	if got != "198.51.100.2" {
		t.Fatalf("extractIP: XFF should be ignored from non-private source, got %q want %q", got, "198.51.100.2")
	}
}

func TestExtractIP_NoTrustProxy(t *testing.T) {
	t.Parallel()
	r := &http.Request{
		RemoteAddr: "198.51.100.2:9999",
		Header:     http.Header{"X-Forwarded-For": {"10.0.0.1, 10.0.0.2"}},
	}
	got := extractIP(r, false)
	if got != "198.51.100.2" {
		t.Fatalf("extractIP: got %q want %q (should ignore X-Forwarded-For)", got, "198.51.100.2")
	}
}

func TestGlobalBucket_AllowBurst(t *testing.T) {
	t.Parallel()
	gb := newGlobalBucket(5, time.Hour)
	for i := 0; i < 5; i++ {
		if !gb.allow() {
			t.Fatalf("request %d: expected allow=true", i+1)
		}
	}
	if gb.allow() {
		t.Fatal("expected allow=false after burst exhausted")
	}
}

func TestGlobalBucket_Recovery(t *testing.T) {
	t.Parallel()
	gb := newGlobalBucket(2, 25*time.Millisecond)
	gb.allow()
	gb.allow()
	if gb.allow() {
		t.Fatal("expected allow=false after burst")
	}
	time.Sleep(30 * time.Millisecond)
	if !gb.allow() {
		t.Fatal("expected allow=true after refill")
	}
}

func TestExtractIP_XForwardedFor_SinglePublicIP(t *testing.T) {
	t.Parallel()
	r := &http.Request{
		RemoteAddr: "10.0.0.1:9999",
		Header:     http.Header{"X-Forwarded-For": {"203.0.113.5"}},
	}
	got := extractIP(r, true)
	if got != "203.0.113.5" {
		t.Fatalf("extractIP: got %q want %q", got, "203.0.113.5")
	}
}

package api

import (
	"strings"
	"testing"
)

func TestValidatePushEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		// Real push services — must be accepted.
		{"FCM (Chrome)", "https://fcm.googleapis.com/fcm/send/abc123:APA91b", false},
		{"Mozilla autopush", "https://updates.push.services.mozilla.com/wpush/v2/gAAAA", false},
		{"Mozilla bare domain", "https://push.services.mozilla.com/wpush/v2/x", false},
		{"WNS (Edge)", "https://db5p.notify.windows.com/w/?token=AwYAAAB", false},
		{"WNS regional subdomain", "https://par02p.notify.windows.com/w/?token=x", false},
		{"Apple web push", "https://web.push.apple.com/QOXxkfmZSt", false},
		{"trailing-dot FQDN", "https://fcm.googleapis.com./fcm/send/x", false},
		{"uppercase host", "https://FCM.GOOGLEAPIS.COM/fcm/send/x", false},

		// SSRF candidates — must be rejected.
		{"loopback IP", "https://127.0.0.1/admin", true},
		{"loopback name", "https://localhost/admin", true},
		{"private IP", "https://10.0.0.5:5432/", true},
		{"link-local metadata IP", "https://169.254.169.254/latest/meta-data/", true},
		{"IPv6 loopback", "https://[::1]/", true},
		{"arbitrary external host", "https://attacker.example.com/collect", true},
		{"lookalike suffix", "https://fcm.googleapis.com.evil.example/", true},
		{"lookalike prefix", "https://evilfcm.googleapis.com.attacker.io/", true},
		{"userinfo trick", "https://fcm.googleapis.com@attacker.example.com/", true},
		{"http scheme", "http://fcm.googleapis.com/fcm/send/x", true},
		{"no scheme", "fcm.googleapis.com/fcm/send/x", true},
		{"empty", "", true},
		{"whitespace URL", "https:// fcm.googleapis.com/x", true},
		{"too long", "https://fcm.googleapis.com/" + strings.Repeat("a", maxPushEndpointLen), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePushEndpoint(tt.endpoint)
			if tt.wantErr && err == nil {
				t.Errorf("validatePushEndpoint(%q) = nil, want error", tt.endpoint)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validatePushEndpoint(%q) = %v, want nil", tt.endpoint, err)
			}
		})
	}
}

func TestHostIsAllowedPushService(t *testing.T) {
	if hostIsAllowedPushService("googleapis.com") {
		t.Error("bare registrable parent of an allowed host must not match")
	}
	if !hostIsAllowedPushService("sub.deep.notify.windows.com") {
		t.Error("nested subdomain of an allowed host must match")
	}
}

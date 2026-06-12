package api

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// maxPushEndpointLen bounds the stored endpoint length.
const maxPushEndpointLen = 2048

// allowedPushHosts is the set of registrable domains operated by the browser
// push services CarWatch supports. An endpoint host must equal one of these or
// be a subdomain of one. This is the primary defense against stored SSRF: the
// notifier POSTs to whatever endpoint we persist, so we only persist URLs that
// point at a real push service.
//
//   - fcm.googleapis.com          — Chrome / Android (FCM)
//   - push.services.mozilla.com   — Firefox (e.g. updates.push.services.mozilla.com)
//   - notify.windows.com          — Edge / WNS (e.g. *.notify.windows.com)
//   - push.apple.com              — Safari (e.g. web.push.apple.com)
var allowedPushHosts = []string{
	"fcm.googleapis.com",
	"push.services.mozilla.com",
	"notify.windows.com",
	"push.apple.com",
}

// validatePushEndpoint checks that raw is a well-formed HTTPS URL pointing at a
// known push service, rejecting anything that could be used to make the server
// issue requests to internal or arbitrary hosts (stored SSRF).
func validatePushEndpoint(raw string) error {
	if raw == "" {
		return fmt.Errorf("endpoint is required")
	}
	if len(raw) > maxPushEndpointLen {
		return fmt.Errorf("endpoint must be at most %d characters", maxPushEndpointLen)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("endpoint must be a valid URL")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("endpoint must be an HTTPS URL")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("endpoint must include a host")
	}

	// Reject IP-literal hosts outright: real push services are addressed by
	// name, and an IP literal is the classic SSRF vector (loopback, link-local,
	// cloud metadata, private ranges).
	if ip := net.ParseIP(host); ip != nil {
		return fmt.Errorf("endpoint host must be a domain name, not an IP address")
	}

	if !hostIsAllowedPushService(host) {
		return fmt.Errorf("endpoint host %q is not a recognized push service", host)
	}
	return nil
}

func hostIsAllowedPushService(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, allowed := range allowedPushHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

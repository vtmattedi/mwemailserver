// MW_ALLOWED_REQUEST_DOMAINS_V1
package requestorigin

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

const MaxRuleLength = 2048

// NormalizeRule canonicalizes a project's allowed_request_domains.
//
// Supported forms:
//   *                  unrestricted
//   *.example.com      any subdomain, but not example.com itself
//   a.com,b.com        either exact hostname
//   example.com,*.example.com
//
// Matching is case-insensitive and ignores the request Origin's port.
func NormalizeRule(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("allowed_request_domains cannot be empty")
	}
	if len(raw) > MaxRuleLength {
		return "", fmt.Errorf("allowed_request_domains exceeds %d bytes", MaxRuleLength)
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			return "", fmt.Errorf("allowed_request_domains contains an empty entry")
		}

		if part == "*" {
			return "*", nil
		}

		if strings.Contains(part, "://") {
			return "", fmt.Errorf(
				"request-domain entries must be hostnames without scheme: %q",
				part,
			)
		}

		if strings.HasPrefix(part, "*.") {
			base := strings.TrimPrefix(part, "*.")
			if net.ParseIP(base) != nil || base == "localhost" {
				return "", fmt.Errorf("wildcard is not valid for %q", base)
			}
			normalized, err := normalizeHostname(base)
			if err != nil {
				return "", fmt.Errorf("invalid wildcard request domain %q: %w", part, err)
			}
			part = "*." + normalized
		} else {
			normalized, err := normalizeHostname(part)
			if err != nil {
				return "", fmt.Errorf("invalid request domain %q: %w", part, err)
			}
			part = normalized
		}

		if _, exists := seen[part]; exists {
			continue
		}

		seen[part] = struct{}{}
		out = append(out, part)
	}

	if len(out) == 0 {
		return "", fmt.Errorf("allowed_request_domains cannot be empty")
	}

	return strings.Join(out, ","), nil
}

// OriginHostname extracts a canonical hostname from an HTTP Origin header.
func OriginHostname(origin string) (string, error) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", fmt.Errorf("origin is empty")
	}
	if origin == "null" {
		return "", fmt.Errorf("opaque origin is not allowed")
	}

	u, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("invalid origin")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("origin hostname is missing")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("origin contains unexpected components")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("origin must not contain a path")
	}

	return normalizeHostname(u.Hostname())
}

func Allowed(rule string, hostname string) (bool, error) {
	rule, err := NormalizeRule(rule)
	if err != nil {
		return false, err
	}

	if rule == "*" {
		return true, nil
	}

	hostname, err = normalizeHostname(hostname)
	if err != nil {
		return false, err
	}

	for _, pattern := range strings.Split(rule, ",") {
		if strings.HasPrefix(pattern, "*.") {
			base := strings.TrimPrefix(pattern, "*.")
			if hostname != base && strings.HasSuffix(hostname, "."+base) {
				return true, nil
			}
			continue
		}

		if hostname == pattern {
			return true, nil
		}
	}

	return false, nil
}

func normalizeHostname(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, ".")

	if value == "" {
		return "", fmt.Errorf("hostname cannot be empty")
	}

	if ip := net.ParseIP(value); ip != nil {
		return ip.String(), nil
	}

	if value == "localhost" {
		return value, nil
	}

	if len(value) > 253 {
		return "", fmt.Errorf("hostname is too long")
	}
	if strings.ContainsAny(value, "/:@[]\\ \t\r\n") {
		return "", fmt.Errorf("hostname contains invalid characters")
	}

	labels := strings.Split(value, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("hostname must contain at least one dot")
	}

	for _, label := range labels {
		if label == "" {
			return "", fmt.Errorf("hostname contains an empty label")
		}
		if len(label) > 63 {
			return "", fmt.Errorf("hostname label is too long")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("hostname label cannot start or end with '-'")
		}

		for _, char := range label {
			if (char >= 'a' && char <= 'z') ||
				(char >= '0' && char <= '9') ||
				char == '-' {
				continue
			}
			return "", fmt.Errorf("hostname contains invalid characters")
		}
	}

	return value, nil
}

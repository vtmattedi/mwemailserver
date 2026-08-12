// MW_ALLOWED_REQUEST_DOMAINS_V1
package requestorigin

import "testing"

func TestNormalizeRule(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"*", "*"},
		{" *.MyDomain.COM ", "*.mydomain.com"},
		{"a.com,b.com", "a.com,b.com"},
		{"A.COM, b.com, a.com", "a.com,b.com"},
		{"mydomain.com,*.mydomain.com", "mydomain.com,*.mydomain.com"},
		{"localhost", "localhost"},
		{"a.com,*", "*"},
	}

	for _, tt := range tests {
		got, err := NormalizeRule(tt.input)
		if err != nil {
			t.Fatalf("NormalizeRule(%q): %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("NormalizeRule(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestOriginHostname(t *testing.T) {
	tests := []struct {
		origin string
		want   string
	}{
		{"https://app.mydomain.com", "app.mydomain.com"},
		{"https://APP.MyDomain.com:443", "app.mydomain.com"},
		{"http://localhost:5173", "localhost"},
	}

	for _, tt := range tests {
		got, err := OriginHostname(tt.origin)
		if err != nil {
			t.Fatalf("OriginHostname(%q): %v", tt.origin, err)
		}
		if got != tt.want {
			t.Fatalf("OriginHostname(%q) = %q; want %q", tt.origin, got, tt.want)
		}
	}
}

func TestAllowed(t *testing.T) {
	tests := []struct {
		rule string
		host string
		want bool
	}{
		{"*", "anything.example", true},
		{"*.mydomain.com", "app.mydomain.com", true},
		{"*.mydomain.com", "deep.app.mydomain.com", true},
		{"*.mydomain.com", "mydomain.com", false},
		{"*.mydomain.com", "notmydomain.com", false},
		{"a.com,b.com", "a.com", true},
		{"a.com,b.com", "b.com", true},
		{"a.com,b.com", "sub.a.com", false},
		{"mydomain.com,*.mydomain.com", "mydomain.com", true},
		{"mydomain.com,*.mydomain.com", "app.mydomain.com", true},
	}

	for _, tt := range tests {
		got, err := Allowed(tt.rule, tt.host)
		if err != nil {
			t.Fatalf("Allowed(%q, %q): %v", tt.rule, tt.host, err)
		}
		if got != tt.want {
			t.Fatalf(
				"Allowed(%q, %q) = %v; want %v",
				tt.rule,
				tt.host,
				got,
				tt.want,
			)
		}
	}
}

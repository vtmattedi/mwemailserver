package federation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxIntrospectionBody = 64 * 1024

type Introspector interface {
	Introspect(context.Context, string) (Principal, error)
}

type Client struct {
	endpoint     string
	clientID     string
	clientSecret string
	audience     string
	httpClient   *http.Client
}

func NewClient(baseURL, clientID, clientSecret, audience string, timeout time.Duration) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("invalid Identity base URL")
	}
	if clientID == "" || clientSecret == "" || audience == "" {
		return nil, fmt.Errorf("Identity client ID, secret and audience are required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("Identity HTTP timeout must be positive")
	}

	return &Client{
		endpoint:     strings.TrimRight(parsed.String(), "/") + "/oauth/introspect",
		clientID:     clientID,
		clientSecret: clientSecret,
		audience:     audience,
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

type stringList []string

func (s *stringList) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*s = list
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*s = []string{single}
	return nil
}

type introspectionResponse struct {
	Active   bool        `json:"active"`
	Subject  string      `json:"sub"`
	Audience stringList  `json:"aud"`
	Scope    string      `json:"scope"`
	AuthTime json.Number `json:"auth_time"`
	AMR      stringList  `json:"amr"`
}

func (c *Client) Introspect(ctx context.Context, token string) (Principal, error) {
	values := url.Values{"token": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return Principal{}, fmt.Errorf("create introspection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.clientID, c.clientSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Principal{}, fmt.Errorf("Identity introspection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Principal{}, fmt.Errorf("Identity introspection returned status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxIntrospectionBody+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return Principal{}, fmt.Errorf("read introspection response: %w", err)
	}
	if len(body) > maxIntrospectionBody {
		return Principal{}, fmt.Errorf("introspection response too large")
	}

	var result introspectionResponse
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return Principal{}, fmt.Errorf("decode introspection response: %w", err)
	}
	if !result.Active || strings.TrimSpace(result.Subject) == "" {
		return Principal{}, ErrInvalidToken
	}
	if !contains(result.Audience, c.audience) {
		return Principal{}, ErrAudienceMismatch
	}

	var authTime time.Time
	if result.AuthTime != "" {
		seconds, err := strconv.ParseInt(string(result.AuthTime), 10, 64)
		if err != nil || seconds < 0 {
			return Principal{}, errors.New("invalid auth_time in introspection response")
		}
		authTime = time.Unix(seconds, 0).UTC()
	}

	return Principal{
		Subject: strings.TrimSpace(result.Subject), Audience: c.audience,
		Scopes: strings.Fields(result.Scope), AuthTime: authTime, AMR: []string(result.AMR),
	}, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

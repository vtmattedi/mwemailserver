package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vtmattedi/mw-email-server/internal/email"
)

const sendURL = "https://api.resend.com/emails"

type Client struct {
	apiKey string
	client *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) Send(
	ctx context.Context,
	message email.Message,
) (*email.SendResult, error) {
	payload := sendRequest{
		From:    message.From,
		To:      message.To,
		Subject: message.Subject,
		HTML:    message.HTML,
		Text:    message.Text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		sendURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create resend request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call resend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		message := strings.TrimSpace(string(data))

		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}

		return nil, fmt.Errorf(
			"resend returned %d: %s",
			resp.StatusCode,
			message,
		)
	}

	var result sendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode resend response: %w", err)
	}

	if result.ID == "" {
		return nil, fmt.Errorf("resend returned an empty email id")
	}

	return &email.SendResult{
		ProviderID: result.ID,
	}, nil
}

package resend

import (
	"net/http"

	svix "github.com/svix/svix-webhooks/go"
)

type WebhookVerifier struct {
	webhook *svix.Webhook
}

func NewWebhookVerifier(secret string) (*WebhookVerifier, error) {
	webhook, err := svix.NewWebhook(secret)
	if err != nil {
		return nil, err
	}

	return &WebhookVerifier{
		webhook: webhook,
	}, nil
}

func (v *WebhookVerifier) Verify(payload []byte, headers http.Header) error {
	return v.webhook.Verify(payload, headers)
}

func DeliveryStatusForEvent(eventType string) string {
	switch eventType {
	case "email.sent":
		return "sent"
	case "email.delivered":
		return "delivered"
	case "email.delivery_delayed":
		return "delivery_delayed"
	case "email.bounced":
		return "bounced"
	case "email.failed":
		return "failed"
	case "email.suppressed":
		return "suppressed"
	case "email.complained":
		return "complained"
	default:
		return ""
	}
}

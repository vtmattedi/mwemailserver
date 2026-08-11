package email

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const MaxIdempotencyKeyLength = 256

func HashIdempotencyKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func HashMessage(message Message) (string, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("marshal request for hashing: %w", err)
	}

	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:]), nil
}

func ProviderIdempotencyKey(projectID int64, keyHash string) string {
	return fmt.Sprintf("mw-email/%d/%s", projectID, keyHash)
}

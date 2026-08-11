package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 2
	argonSaltLen        = 16
	argonKeyLen  uint32 = 32

	minPasswordBytes = 12
	maxPasswordBytes = 256
)

var ErrInvalidPasswordHash = errors.New("invalid password hash")

func ValidatePassword(password string) error {
	length := len([]byte(password))

	if length < minPasswordBytes {
		return fmt.Errorf("password must be at least %d bytes", minPasswordBytes)
	}

	if length > maxPasswordBytes {
		return fmt.Errorf("password must be at most %d bytes", maxPasswordBytes)
	}

	return nil
}

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		argonKeyLen,
	)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonTime,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(password string, encoded string) (bool, error) {
	params, salt, expected, err := parsePasswordHash(encoded)
	if err != nil {
		return false, err
	}

	actual := argon2.IDKey(
		[]byte(password),
		salt,
		params.time,
		params.memory,
		params.threads,
		uint32(len(expected)),
	)

	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

func parsePasswordHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, ErrInvalidPasswordHash
	}

	versionText := strings.TrimPrefix(parts[2], "v=")
	version, err := strconv.Atoi(versionText)
	if err != nil || version != argon2.Version {
		return argonParams{}, nil, nil, ErrInvalidPasswordHash
	}

	var params argonParams
	for _, entry := range strings.Split(parts[3], ",") {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			return argonParams{}, nil, nil, ErrInvalidPasswordHash
		}

		number, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return argonParams{}, nil, nil, ErrInvalidPasswordHash
		}

		switch key {
		case "m":
			params.memory = uint32(number)
		case "t":
			params.time = uint32(number)
		case "p":
			if number == 0 || number > 255 {
				return argonParams{}, nil, nil, ErrInvalidPasswordHash
			}
			params.threads = uint8(number)
		default:
			return argonParams{}, nil, nil, ErrInvalidPasswordHash
		}
	}

	if params.memory == 0 || params.time == 0 || params.threads == 0 {
		return argonParams{}, nil, nil, ErrInvalidPasswordHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return argonParams{}, nil, nil, ErrInvalidPasswordHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) < 16 {
		return argonParams{}, nil, nil, ErrInvalidPasswordHash
	}

	return params, salt, hash, nil
}

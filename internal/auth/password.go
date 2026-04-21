package auth

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
	passwordAlgorithmArgon2ID = "argon2id"
	passwordSaltLength        = 16
	passwordKeyLength         = 32
	passwordTimeCost          = 3
	passwordMemoryCost        = 64 * 1024
	passwordParallelism       = 2
)

var (
	// ErrPasswordHashFormat 表示密码哈希串格式非法。
	ErrPasswordHashFormat = errors.New("password hash format is invalid")
)

// HashPassword 使用 argon2id 生成密码哈希。
func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey(
		[]byte(password),
		salt,
		passwordTimeCost,
		passwordMemoryCost,
		passwordParallelism,
		passwordKeyLength,
	)
	return fmt.Sprintf(
		"$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		passwordAlgorithmArgon2ID,
		argon2.Version,
		passwordMemoryCost,
		passwordTimeCost,
		passwordParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword 校验明文密码与 argon2id 哈希是否匹配。
func VerifyPassword(password string, encoded string) (bool, error) {
	algorithm, version, memory, timeCost, parallelism, salt, hashValue, err := decodePasswordHash(encoded)
	if err != nil {
		return false, err
	}
	if algorithm != passwordAlgorithmArgon2ID || version != argon2.Version {
		return false, ErrPasswordHashFormat
	}
	computed := argon2.IDKey(
		[]byte(password),
		salt,
		timeCost,
		memory,
		parallelism,
		uint32(len(hashValue)),
	)
	return subtle.ConstantTimeCompare(computed, hashValue) == 1, nil
}

func decodePasswordHash(encoded string) (string, int, uint32, uint32, uint8, []byte, []byte, error) {
	parts := strings.Split(strings.TrimSpace(encoded), "$")
	if len(parts) != 6 || parts[0] != "" {
		return "", 0, 0, 0, 0, nil, nil, ErrPasswordHashFormat
	}

	versionValue, err := parseHashInt(strings.TrimPrefix(parts[2], "v="))
	if err != nil {
		return "", 0, 0, 0, 0, nil, nil, ErrPasswordHashFormat
	}

	var memory uint64
	var timeCost uint64
	var parallelism uint64
	for _, item := range strings.Split(parts[3], ",") {
		key, value, found := strings.Cut(item, "=")
		if !found {
			return "", 0, 0, 0, 0, nil, nil, ErrPasswordHashFormat
		}
		parsed, parseErr := parseHashInt(value)
		if parseErr != nil {
			return "", 0, 0, 0, 0, nil, nil, ErrPasswordHashFormat
		}
		switch key {
		case "m":
			memory = uint64(parsed)
		case "t":
			timeCost = uint64(parsed)
		case "p":
			parallelism = uint64(parsed)
		default:
			return "", 0, 0, 0, 0, nil, nil, ErrPasswordHashFormat
		}
	}
	if memory == 0 || timeCost == 0 || parallelism == 0 {
		return "", 0, 0, 0, 0, nil, nil, ErrPasswordHashFormat
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return "", 0, 0, 0, 0, nil, nil, ErrPasswordHashFormat
	}
	hashValue, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hashValue) == 0 {
		return "", 0, 0, 0, 0, nil, nil, ErrPasswordHashFormat
	}

	return parts[1], versionValue, uint32(memory), uint32(timeCost), uint8(parallelism), salt, hashValue, nil
}

func parseHashInt(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, ErrPasswordHashFormat
	}
	return value, nil
}

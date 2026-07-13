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

type passwordHash struct {
	algorithm   string
	version     int
	memory      uint32
	timeCost    uint32
	parallelism uint8
	salt        []byte
	value       []byte
}

type passwordHashParameters struct {
	memory      uint64
	timeCost    uint64
	parallelism uint64
}

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
	decoded, err := decodePasswordHash(encoded)
	if err != nil {
		return false, err
	}
	if decoded.algorithm != passwordAlgorithmArgon2ID || decoded.version != argon2.Version {
		return false, ErrPasswordHashFormat
	}
	computed := argon2.IDKey(
		[]byte(password),
		decoded.salt,
		decoded.timeCost,
		decoded.memory,
		decoded.parallelism,
		uint32(len(decoded.value)),
	)
	return subtle.ConstantTimeCompare(computed, decoded.value) == 1, nil
}

func decodePasswordHash(encoded string) (passwordHash, error) {
	parts := strings.Split(strings.TrimSpace(encoded), "$")
	if len(parts) != 6 || parts[0] != "" {
		return passwordHash{}, ErrPasswordHashFormat
	}
	versionValue, err := parseHashInt(strings.TrimPrefix(parts[2], "v="))
	if err != nil {
		return passwordHash{}, ErrPasswordHashFormat
	}
	parameters, err := parsePasswordHashParameters(parts[3])
	if err != nil {
		return passwordHash{}, err
	}
	salt, err := decodePasswordHashBytes(parts[4])
	if err != nil {
		return passwordHash{}, err
	}
	hashValue, err := decodePasswordHashBytes(parts[5])
	if err != nil {
		return passwordHash{}, err
	}
	return passwordHash{
		algorithm:   parts[1],
		version:     versionValue,
		memory:      uint32(parameters.memory),
		timeCost:    uint32(parameters.timeCost),
		parallelism: uint8(parameters.parallelism),
		salt:        salt,
		value:       hashValue,
	}, nil
}

func parsePasswordHashParameters(raw string) (passwordHashParameters, error) {
	parameters := passwordHashParameters{}
	for _, item := range strings.Split(raw, ",") {
		key, value, found := strings.Cut(item, "=")
		if !found {
			return passwordHashParameters{}, ErrPasswordHashFormat
		}
		parsed, parseErr := parseHashInt(value)
		if parseErr != nil {
			return passwordHashParameters{}, ErrPasswordHashFormat
		}
		switch key {
		case "m":
			parameters.memory = uint64(parsed)
		case "t":
			parameters.timeCost = uint64(parsed)
		case "p":
			parameters.parallelism = uint64(parsed)
		default:
			return passwordHashParameters{}, ErrPasswordHashFormat
		}
	}
	if parameters.memory == 0 || parameters.memory > uint64(^uint32(0)) ||
		parameters.timeCost == 0 || parameters.timeCost > uint64(^uint32(0)) ||
		parameters.parallelism == 0 || parameters.parallelism > uint64(^uint8(0)) {
		return passwordHashParameters{}, ErrPasswordHashFormat
	}
	return parameters, nil
}

func decodePasswordHashBytes(raw string) ([]byte, error) {
	value, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil || len(value) == 0 {
		return nil, ErrPasswordHashFormat
	}
	return value, nil
}

func parseHashInt(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, ErrPasswordHashFormat
	}
	return value, nil
}

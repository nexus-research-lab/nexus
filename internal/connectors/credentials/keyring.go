// INPUT: 一把 active Connector credentials key、显式 legacy keys、持久 key_id 与加密载荷。
// OUTPUT: 去重后的 keyring、稳定 key_id、按身份精确解密及无身份旧载荷的有限兼容解密。
// POS: Connector connection 凭据密钥识别与轮换的唯一密码学边界；业务 service 不选择密钥来源。
package credentials

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const identifiedEnvelopePrefix = "v2."

var (
	// ErrKeyUnavailable 表示载荷声明的持久 key_id 不在当前宿主 keyring 中。
	ErrKeyUnavailable = errors.New("connector credentials key 不可用")
	// ErrNoMatchingKey 表示无 key_id 的旧载荷无法由任何显式候选密钥解密。
	ErrNoMatchingKey = errors.New("connector credentials 旧载荷没有匹配密钥")
)

type keyringEntry struct {
	id  string
	key []byte
}

// Keyring 固定一把写入密钥，并保留有限、显式的只读历史密钥集合。
type Keyring struct {
	active  keyringEntry
	byID    map[string]keyringEntry
	ordered []keyringEntry
}

// NewKeyring 解析并去重 active/legacy keys；active key 始终是新写入的唯一密钥。
func NewKeyring(activeRaw string, legacyRaw []string) (*Keyring, error) {
	activeKey, err := DecodeKey(activeRaw)
	if err != nil {
		return nil, err
	}
	active := keyringEntry{id: KeyID(activeKey), key: activeKey}
	keyring := &Keyring{
		active:  active,
		byID:    map[string]keyringEntry{active.id: active},
		ordered: []keyringEntry{active},
	}
	for index, raw := range legacyRaw {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		key, decodeErr := DecodeKey(raw)
		if decodeErr != nil {
			return nil, fmt.Errorf("解析 legacy connector credentials key %d: %w", index+1, decodeErr)
		}
		entry := keyringEntry{id: KeyID(key), key: key}
		if _, exists := keyring.byID[entry.id]; exists {
			continue
		}
		keyring.byID[entry.id] = entry
		keyring.ordered = append(keyring.ordered, entry)
	}
	return keyring, nil
}

// KeyID 返回不包含秘密的稳定 SHA-256 密钥身份。
func KeyID(key []byte) string {
	digest := sha256.Sum256(key)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// ActiveKeyID 返回所有新密文必须持久化的 key_id。
func (k *Keyring) ActiveKeyID() string {
	if k == nil {
		return ""
	}
	return k.active.id
}

// Encrypt 使用唯一 active key 加密，并返回必须与密文同事务保存的 key_id。
func (k *Keyring) Encrypt(payload []byte) (encrypted string, keyID string, err error) {
	if k == nil || len(k.active.key) == 0 {
		return "", "", ErrKeyUnavailable
	}
	encrypted, err = EncryptPayload(k.active.key, payload)
	if err != nil {
		return "", "", err
	}
	return encrypted, k.active.id, nil
}

// Decrypt 按持久 key_id 精确解密；只有旧记录缺少 key_id 时才有限尝试显式 keyring。
func (k *Keyring) Decrypt(keyID string, encrypted string) (payload []byte, matchedKeyID string, err error) {
	if k == nil {
		return nil, "", ErrKeyUnavailable
	}
	keyID = strings.TrimSpace(keyID)
	if keyID != "" {
		entry, ok := k.byID[keyID]
		if !ok {
			return nil, "", fmt.Errorf("%w: %s", ErrKeyUnavailable, keyID)
		}
		payload, err = DecryptPayload(entry.key, encrypted)
		return payload, entry.id, err
	}
	for _, entry := range k.ordered {
		payload, decryptErr := DecryptPayload(entry.key, encrypted)
		if decryptErr == nil {
			return payload, entry.id, nil
		}
	}
	return nil, "", ErrNoMatchingKey
}

// EncryptEnvelope 为没有独立 key_id 列的持久字段生成自带密钥身份的载荷。
func (k *Keyring) EncryptEnvelope(payload []byte) (string, error) {
	encrypted, keyID, err := k.Encrypt(payload)
	if err != nil {
		return "", err
	}
	encodedKeyID := base64.RawURLEncoding.EncodeToString([]byte(keyID))
	return identifiedEnvelopePrefix + encodedKeyID + "." + encrypted, nil
}

// DecryptEnvelope 精确读取新载荷；只有没有身份的旧 v1 载荷才尝试显式 legacy keyring。
func (k *Keyring) DecryptEnvelope(envelope string) ([]byte, error) {
	envelope = strings.TrimSpace(envelope)
	if !strings.HasPrefix(envelope, identifiedEnvelopePrefix) {
		payload, _, err := k.Decrypt("", envelope)
		return payload, err
	}
	parts := strings.SplitN(strings.TrimPrefix(envelope, identifiedEnvelopePrefix), ".", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return nil, errors.New("connector credentials identified envelope 格式不正确")
	}
	keyIDBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	keyID := string(keyIDBytes)
	digest := strings.TrimPrefix(keyID, "sha256:")
	_, digestErr := hex.DecodeString(digest)
	if err != nil || len(digest) != sha256.Size*2 || digestErr != nil || !strings.HasPrefix(keyID, "sha256:") {
		return nil, errors.New("connector credentials identified envelope key_id 格式不正确")
	}
	payload, _, err := k.Decrypt(keyID, parts[1])
	return payload, err
}

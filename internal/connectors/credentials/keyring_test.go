package credentials

import (
	"errors"
	"testing"
)

const (
	testActiveKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	testLegacyKey = "YWJjZGVmMDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODk="
)

func TestKeyringUsesStableIdentityAndOneActiveWriter(t *testing.T) {
	keyring, err := NewKeyring(testActiveKey, []string{testLegacyKey, testActiveKey})
	if err != nil {
		t.Fatal(err)
	}
	encrypted, keyID, err := keyring.Encrypt([]byte("current"))
	if err != nil {
		t.Fatal(err)
	}
	if keyID == "" || keyID != keyring.ActiveKeyID() || len(keyring.ordered) != 2 {
		t.Fatalf("keyring identity/dedup mismatch: key_id=%q entries=%d", keyID, len(keyring.ordered))
	}
	plain, matched, err := keyring.Decrypt(keyID, encrypted)
	if err != nil || string(plain) != "current" || matched != keyID {
		t.Fatalf("active decrypt mismatch: plain=%q matched=%q err=%v", plain, matched, err)
	}
}

func TestKeyringReadsUnidentifiedLegacyPayloadWithoutGuessingAfterIdentification(t *testing.T) {
	legacyKey, err := DecodeKey(testLegacyKey)
	if err != nil {
		t.Fatal(err)
	}
	legacyPayload, err := EncryptPayload(legacyKey, []byte("legacy"))
	if err != nil {
		t.Fatal(err)
	}
	keyring, err := NewKeyring(testActiveKey, []string{testLegacyKey})
	if err != nil {
		t.Fatal(err)
	}
	plain, matched, err := keyring.Decrypt("", legacyPayload)
	if err != nil || string(plain) != "legacy" || matched == keyring.ActiveKeyID() {
		t.Fatalf("legacy decrypt mismatch: plain=%q matched=%q err=%v", plain, matched, err)
	}
	if _, _, err = keyring.Decrypt("sha256:missing", legacyPayload); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("identified payload must not scan other keys: %v", err)
	}
}

func TestKeyringRejectsUnknownLegacyPayload(t *testing.T) {
	unknownKey, err := DecodeKey("enl4d3Z1dHNycXBvbm1sa2ppaGdmZWRjYmEwMTIzNDU=")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := EncryptPayload(unknownKey, []byte("unknown"))
	if err != nil {
		t.Fatal(err)
	}
	keyring, err := NewKeyring(testActiveKey, []string{testLegacyKey})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = keyring.Decrypt("", encrypted); !errors.Is(err, ErrNoMatchingKey) {
		t.Fatalf("unknown legacy key must fail closed: %v", err)
	}
}

func TestKeyringEnvelopeUsesExactIdentityAndReadsLegacyV1(t *testing.T) {
	keyring, err := NewKeyring(testActiveKey, []string{testLegacyKey})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := keyring.EncryptEnvelope([]byte("current"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := keyring.DecryptEnvelope(envelope)
	if err != nil || string(plain) != "current" {
		t.Fatalf("identified envelope mismatch: plain=%q err=%v", plain, err)
	}

	legacyKey, err := DecodeKey(testLegacyKey)
	if err != nil {
		t.Fatal(err)
	}
	legacyPayload, err := EncryptPayload(legacyKey, []byte("legacy"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err = keyring.DecryptEnvelope(legacyPayload)
	if err != nil || string(plain) != "legacy" {
		t.Fatalf("legacy v1 envelope mismatch: plain=%q err=%v", plain, err)
	}

	unknown, err := NewKeyring("enl4d3Z1dHNycXBvbm1sa2ppaGdmZWRjYmEwMTIzNDU=", nil)
	if err != nil {
		t.Fatal(err)
	}
	unknownEnvelope, err := unknown.EncryptEnvelope([]byte("unknown"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = keyring.DecryptEnvelope(unknownEnvelope); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("identified unknown envelope must fail exact: %v", err)
	}
}

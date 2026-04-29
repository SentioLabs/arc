package paste

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	in := map[string]any{"hello": "world", "n": float64(42)}
	ct, iv, err := EncryptJSON(in, key)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := DecryptJSON(ct, iv, key, &out); err != nil {
		t.Fatalf("DecryptJSON: %v", err)
	}
	if out["hello"] != "world" {
		t.Errorf("roundtrip mismatch: %+v", out)
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	k1, _ := GenerateKey()
	k2, _ := GenerateKey()
	ct, iv, _ := EncryptJSON("secret", k1)
	var out string
	if err := DecryptJSON(ct, iv, k2, &out); err == nil {
		t.Error("expected decrypt to fail with wrong key")
	}
}

func TestKeySizeValidation(t *testing.T) {
	short := bytes.Repeat([]byte{1}, 16)
	if _, _, err := EncryptJSON("x", short); err == nil {
		t.Error("expected error for short key")
	}
}

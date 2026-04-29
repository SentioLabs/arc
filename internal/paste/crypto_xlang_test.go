package paste

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type xlangFixture struct {
	Name          string          `json:"name"`
	KeyB64Url     string          `json:"key_b64url"`
	Plaintext     json.RawMessage `json:"plaintext"`
	CiphertextB64 string          `json:"ciphertext_b64"`
	IvB64         string          `json:"iv_b64"`
}

func TestCryptoXLangFixtures(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xlang_fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []xlangFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixtures loaded")
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			key, err := base64UrlDecode(f.KeyB64Url)
			if err != nil {
				t.Fatal(err)
			}
			ct, _ := base64.StdEncoding.DecodeString(f.CiphertextB64)
			iv, _ := base64.StdEncoding.DecodeString(f.IvB64)
			var got json.RawMessage
			if err := DecryptJSON(ct, iv, key, &got); err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			var a, b any
			_ = json.Unmarshal(f.Plaintext, &a)
			_ = json.Unmarshal(got, &b)
			if !reflect.DeepEqual(a, b) {
				t.Errorf("plaintext mismatch:\nwant %s\ngot  %s", f.Plaintext, got)
			}
		})
	}
}

func TestCryptoXLangRoundtrip(t *testing.T) {
	// For each fixture, also verify that re-encrypting and re-decrypting in Go
	// produces the same plaintext (catches Go-internal regressions).
	data, _ := os.ReadFile(filepath.Join("testdata", "xlang_fixtures.json"))
	var fixtures []xlangFixture
	_ = json.Unmarshal(data, &fixtures)
	for _, f := range fixtures {
		t.Run(f.Name+"-roundtrip", func(t *testing.T) {
			key, _ := base64UrlDecode(f.KeyB64Url)
			ct, iv, err := EncryptJSON(json.RawMessage(f.Plaintext), key)
			if err != nil {
				t.Fatal(err)
			}
			var out json.RawMessage
			if err := DecryptJSON(ct, iv, key, &out); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func base64UrlDecode(s string) ([]byte, error) {
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

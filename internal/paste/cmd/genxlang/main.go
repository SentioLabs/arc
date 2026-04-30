// genxlang generates testdata/xlang_fixtures.json with Go-encrypted blobs.
// Run once to populate; the fixtures are checked into the repo and used
// by both Go and TS tests to verify cross-language compatibility.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sentiolabs/arc/internal/paste"
)

type fixture struct {
	Name          string `json:"name"`
	KeyB64Url     string `json:"key_b64url"`
	Plaintext     any    `json:"plaintext"`
	CiphertextB64 string `json:"ciphertext_b64"`
	IvB64         string `json:"iv_b64"`
}

func main() {
	cases := []struct {
		name string
		v    any
	}{
		{"simple-string", "hello world"},
		{"empty-object", map[string]any{}},
		{"nested-object", map[string]any{
			"kind": "comment", "id": "c1", "author_name": "Alice",
			"anchor": map[string]any{"line_start": 1, "line_end": 1, "quoted_text": "x"},
		}},
	}
	out := make([]fixture, 0, len(cases))
	for _, c := range cases {
		key, _ := paste.GenerateKey()
		ct, iv, err := paste.EncryptJSON(c.v, key)
		if err != nil {
			panic(err)
		}
		out = append(out, fixture{
			Name:          c.name,
			KeyB64Url:     base64.RawURLEncoding.EncodeToString(key),
			Plaintext:     c.v,
			CiphertextB64: base64.StdEncoding.EncodeToString(ct),
			IvB64:         base64.StdEncoding.EncodeToString(iv),
		})
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(data))
	const fixtureMode = 0o600
	_ = os.WriteFile("internal/paste/testdata/xlang_fixtures.json", data, fixtureMode)
}

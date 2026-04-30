# Crypto cross-language fixtures

`xlang_fixtures.json` contains AES-256-GCM ciphertexts produced by the Go
implementation in `internal/paste/crypto.go`. The TypeScript test
`web/src/lib/paste/crypto.xlang.test.ts` reads these fixtures and verifies
that the JS Web Crypto API decrypts them to the original plaintext.

## Regenerate

    go run ./internal/paste/cmd/genxlang/

This overwrites the JSON file with fresh ciphertexts (random keys + IVs each
run). Don't regenerate casually — the goal is for both Go and TS tests to
pass against the same checked-in fixtures, so a regen invalidates the
TS-side check until you also re-run it.

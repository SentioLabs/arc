// Package workspace provides utilities for workspace management.
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// base36Chars defines the character set for base36 encoding.
const base36Chars = "0123456789abcdefghijklmnopqrstuvwxyz"

// Base36Encode converts bytes to a base36 string.
func Base36Encode(data []byte) string {
	n := new(big.Int).SetBytes(data)
	if n.Sign() == 0 {
		return "0"
	}

	base := big.NewInt(36)
	var result []byte
	mod := new(big.Int)

	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		result = append([]byte{base36Chars[mod.Int64()]}, result...)
	}

	return string(result)
}

// GenerateIssueID creates an issue ID from content.
// Format: prefix-{6-char-base36-hash}
func GenerateIssueID(prefix, title string) string {
	h := sha256.Sum256([]byte(title + time.Now().String()))
	// Use first 3 bytes for ~5-6 base36 characters
	encoded := Base36Encode(h[:3])

	// Pad to exactly 6 chars
	for len(encoded) < 6 {
		encoded = "0" + encoded
	}
	// Trim if longer (shouldn't happen with 3 bytes, but be safe)
	if len(encoded) > 6 {
		encoded = encoded[:6]
	}

	return prefix + "-" + encoded
}

// GenerateName creates a workspace name from a directory path.
// Format: {sanitized-basename}-{6-char-hex-hash}
// The hash is derived from the full absolute path, making it deterministic.
func GenerateName(dirPath string) (string, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", err
	}

	// Resolve symlinks for consistency
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// Fall back to absPath if symlink resolution fails (e.g., path doesn't exist yet)
		evalPath = absPath
	}

	// Normalize path separators for cross-platform consistency
	normalized := filepath.ToSlash(evalPath)

	// Get sanitized basename
	basename := SanitizeBasename(filepath.Base(evalPath))

	// Generate deterministic hash from full path
	hash := sha256.Sum256([]byte(normalized))
	suffix := hex.EncodeToString(hash[:])[:6]

	return basename + "-" + suffix, nil
}

// SanitizeBasename normalizes a directory name for use in workspace names.
func SanitizeBasename(name string) string {
	// Lowercase
	name = strings.ToLower(name)

	// Replace spaces and underscores with hyphens
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove non-alphanumeric characters except hyphens
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = re.ReplaceAllString(name, "")

	// Collapse multiple consecutive hyphens
	re = regexp.MustCompile(`-+`)
	name = re.ReplaceAllString(name, "-")

	// Trim hyphens from ends
	name = strings.Trim(name, "-")

	// Truncate if too long (keep room for -xxxxxx hash suffix)
	if len(name) > 20 {
		name = name[:20]
	}

	// Fallback for empty result
	if name == "" {
		name = "workspace"
	}

	return name
}

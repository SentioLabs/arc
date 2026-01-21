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
// Format: prefix.{6-char-base36-hash}
// Uses period separator to distinguish from workspace prefixes which use hyphens.
func GenerateIssueID(prefix, title string) string {
	return generateHashID(prefix, title, ".")
}

// GenerateWorkspaceID creates a workspace ID from content.
// Format: prefix-{6-char-base36-hash}
// Uses hyphen separator consistent with workspace naming conventions.
func GenerateWorkspaceID(prefix, name string) string {
	return generateHashID(prefix, name, "-")
}

// generateHashID is the internal function that creates hash-based IDs.
// Format: prefix{separator}{6-char-base36-hash}
func generateHashID(prefix, content, separator string) string {
	h := sha256.Sum256([]byte(content + time.Now().String()))
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

	return prefix + separator + encoded
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

// GeneratePrefix creates an issue prefix from a directory path.
// Format: {alphanumeric-basename-truncated}-{4-char-base36-hash}
// The hash is derived from the full absolute path, making it deterministic.
// Example: /home/user/projects/my-api -> "myapi-a3f2"
func GeneratePrefix(dirPath string) (string, error) {
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

	// Get alphanumeric-only basename, truncated for prefix use
	// Max 5 chars to fit in 10-char limit: 5 basename + 1 hyphen + 4 suffix = 10
	basename := normalizeForPrefix(filepath.Base(evalPath))
	if len(basename) > 5 {
		basename = basename[:5]
	}

	// Generate deterministic hash from full path using base36
	hash := sha256.Sum256([]byte(normalized))
	suffix := Base36Encode(hash[:2]) // 2 bytes -> ~3-4 base36 chars

	// Ensure exactly 4 chars for the suffix
	for len(suffix) < 4 {
		suffix = "0" + suffix
	}
	if len(suffix) > 4 {
		suffix = suffix[:4]
	}

	return basename + "-" + suffix, nil
}

// GeneratePrefixFromName creates an issue prefix from a workspace name (without path).
// Format: {alphanumeric-name-truncated}-{4-char-base36-hash}
// Used when creating a workspace without an associated directory path.
// Includes timestamp for uniqueness when same name is used multiple times.
func GeneratePrefixFromName(name string) string {
	// Normalize to alphanumeric only and truncate
	// Max 5 chars to fit in 10-char limit: 5 basename + 1 hyphen + 4 suffix = 10
	normalized := normalizeForPrefix(name)
	if len(normalized) > 5 {
		normalized = normalized[:5]
	}

	// Generate hash from name + timestamp for uniqueness
	hash := sha256.Sum256([]byte(name + time.Now().String()))
	suffix := Base36Encode(hash[:2])

	// Ensure exactly 4 chars for the suffix
	for len(suffix) < 4 {
		suffix = "0" + suffix
	}
	if len(suffix) > 4 {
		suffix = suffix[:4]
	}

	return normalized + "-" + suffix
}

// normalizeForPrefix converts a name to lowercase alphanumeric only.
// All special characters (including hyphens, spaces, underscores) are removed.
// Example: "My-Cool_Project!" -> "mycoolproject"
func normalizeForPrefix(name string) string {
	// Lowercase
	name = strings.ToLower(name)

	// Keep only alphanumeric characters
	re := regexp.MustCompile(`[^a-z0-9]`)
	name = re.ReplaceAllString(name, "")

	// Fallback for empty result
	if name == "" {
		name = "ws"
	}

	return name
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

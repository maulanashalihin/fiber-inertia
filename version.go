package fiberinertia

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// ---------------------------------------------------------------------------
// Version helpers
//
// Inertia uses an asset version string to detect when client-side assets
// have changed. When the version mismatches, the server returns 409,
// forcing a full page reload so the user gets the latest assets.
//
// These helpers provide common strategies for generating a version string.
// ---------------------------------------------------------------------------

// VersionFromFile returns the contents of path as the version string,
// with leading/trailing whitespace trimmed. Useful when using a file
// that contains a hash (e.g. from a CI build step).
//
// Returns an empty string and no error if the file doesn't exist.
func VersionFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("fiberinertia: reading version file: %w", err)
	}
	return string(data), nil // caller should trim spaces
}

// VersionFromFileHash reads the file at path and returns its SHA-256
// hex digest. Useful for automatic cache busting: when the file changes,
// the version string changes.
func VersionFromFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("fiberinertia: hashing version file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 32*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// VersionFromEnv returns the value of the environment variable named by key.
// Returns an empty string if the variable is unset or empty.
func VersionFromEnv(key string) string {
	return os.Getenv(key)
}

package store

import "io"

// Store defines a persistent key/value store.
type Store interface {
	io.Closer

	// Get retrieves the value for a key. Returns an error if the key does not exist.
	Get(key string) (string, error)

	// Set stores a key/value pair, creating or overwriting as needed.
	Set(key, value string) error

	// Delete removes a key. Idempotent — no error if the key does not exist.
	Delete(key string) error

	// List returns keys matching the given prefix. An empty prefix returns all keys.
	// Returns at most limit keys (0 means no limit).
	List(prefix string, limit int) ([]string, error)
}

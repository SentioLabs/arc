package config

import "sync/atomic"

// Store wraps an atomically-swappable *Config. The arc server holds one
// Store and reads through Load() so that a future Reload() can hot-swap
// without mutating consumers. Callers must treat the returned *Config
// as read-only.
type Store struct {
	v atomic.Pointer[Config]
}

// NewStore creates a Store pre-loaded with cfg.
func NewStore(cfg *Config) *Store {
	s := &Store{}
	s.v.Store(cfg)
	return s
}

// Load returns the current config snapshot.
func (s *Store) Load() *Config { return s.v.Load() }

// Swap atomically replaces the active config and returns the previous one.
func (s *Store) Swap(cfg *Config) *Config { return s.v.Swap(cfg) }

package auth

import (
	"context"

	"workshop/api/internal/store"
)

// SeedFirstEmployee creates the first workshop employee from configuration if
// no employee exists yet. The real implementation (password hashing and the
// employee store) belongs to the workshop-login ticket; this skeleton is a
// no-op so the startup sequence is complete and callable.
func SeedFirstEmployee(_ context.Context, _ *store.Store) error {
	return nil
}

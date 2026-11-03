package auth

import (
	"context"
	"fmt"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
)

// SetupNeeded reports whether first-run setup is still open: true until the first user exists.
func (a *Auth) SetupNeeded(ctx context.Context, q db.DBTX) (bool, error) {
	exists, err := sqlc.New(q).UserExists(ctx)
	if err != nil {
		return false, fmt.Errorf("check for a user: %w", err)
	}
	return !exists, nil
}

package outbound

import (
	"github.com/google/uuid"
)

// TokenIssuer is the port used by the adapters needing this
type TokenIssuer interface {
	Issue(userID uuid.UUID) (string, error)
	Verify(token string) (uuid.UUID, error)
}

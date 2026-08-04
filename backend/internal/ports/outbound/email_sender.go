package outbound

import (
	"context"
	"errors"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

// ErrInvalidMessage marks a Send failure as deterministic: the message itself
// is unsendable, so the same call will fail the same way. Implementations wrap
// it around message-construction failures and return transient failures bare.
//
//nolint:revive // sentinel error
var ErrInvalidMessage = errors.New("invalid message")

// EmailSenderParams contains all the fields for composing an email
type EmailSenderParams struct {
	To      shared.Email
	ReplyTo shared.Email
	Subject string
	Message string
}

// EmailSender is the interface containing methods for sending system emails
type EmailSender interface {
	Send(ctx context.Context, params EmailSenderParams) error
}

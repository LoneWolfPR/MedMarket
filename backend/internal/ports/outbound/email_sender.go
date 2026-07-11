package outbound

import (
	"context"

	"github.com/LoneWolfPR/MedMarket/backend/internal/domain/shared"
)

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

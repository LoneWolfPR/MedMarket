// Package mailer is for sending email communications through the system
package mailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ports/outbound"
)

// Mailer is the struct containing the necessary properties
// for sending email notifications
type Mailer struct {
	logger *slog.Logger
	auth   smtp.Auth
	addr   string
	from   string
}

var _ outbound.EmailSender = (*Mailer)(nil)

// NewMailerParams contains the parameters for constructing a
// new instance of Mailer
type NewMailerParams struct {
	Logger *slog.Logger
	Auth   smtp.Auth
	Addr   string
	From   string
}

// NewMailer is the constructor for building a new Mailer instance
func NewMailer(p NewMailerParams) (*Mailer, error) {
	if p.Logger == nil {
		return nil, errors.New("logger is missing")
	}
	if p.Addr == "" {
		return nil, errors.New("smtp host address is missing")
	}
	if p.From == "" {
		return nil, errors.New("from address is missing")
	}

	return &Mailer{
		logger: p.Logger,
		auth:   p.Auth,
		addr:   p.Addr,
		from:   p.From,
	}, nil
}

//nolint:revive // sentinel error
var ErrInvalidMessage = errors.New("invalid message")

// Send will take a list of necessary parameters and send an email to the specified
// recipient
func (m *Mailer) Send(ctx context.Context, params outbound.EmailSenderParams) error {
	to := []string{params.To.String()}

	msg, err := m.buildMessage(params)
	if err != nil {
		return fmt.Errorf("error constructing message: %w: %w", ErrInvalidMessage, err)
	}
	err = smtp.SendMail(m.addr, m.auth, m.from, to, []byte(msg))
	if err != nil {
		return fmt.Errorf("error sending mail: %w", err)
	}

	m.logger.DebugContext(ctx, "email successfully sent to", "recipient", params.To.String())
	return nil
}

func (m *Mailer) buildMessage(params outbound.EmailSenderParams) (string, error) {
	if strings.ContainsAny(params.Subject, "\r\n") {
		return "", errors.New("subject contains illegal characters")
	}
	msg := "From: " + m.from + "\r\n"
	msg += "To: " + params.To.String() + "\r\n"
	if !params.ReplyTo.IsZero() {
		msg += "Reply-To: " + params.ReplyTo.String() + "\r\n"
	}
	msg += "Subject: " + params.Subject + "\r\n"
	msg += "Content-Type: text/plain; charset=UTF-8" + "\r\n\r\n"
	msg += params.Message
	return msg, nil
}

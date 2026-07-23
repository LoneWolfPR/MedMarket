package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Handler is a fanout handler for sending telemtry to multiple slog handlers
type Handler struct {
	handlers []slog.Handler
	minLevel slog.Level
}

var _ slog.Handler = Handler{}

// NewHandler constructs the handler
func NewHandler(newHandlers []slog.Handler, level slog.Level) (Handler, error) {
	var errs []error
	handlers := []slog.Handler{}
	for idx, handler := range newHandlers {
		if handler == nil {
			errs = append(errs, fmt.Errorf("handler at index %d is nil", idx))
		} else {
			handlers = append(handlers, handler)
		}
	}
	if len(handlers) == 0 {
		errs = append(errs, errors.New("no handlers found"))
	}
	if len(errs) > 0 {
		return Handler{}, errors.Join(errs...)
	}
	return Handler{
		handlers: handlers,
		minLevel: level,
	}, nil
}

// Enabled checks if any of the handlers are enabled for the given log level
func (h Handler) Enabled(ctx context.Context, level slog.Level) bool {
	if level < h.minLevel {
		return false
	}
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle runs the handle method on all passed handlers
func (h Handler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level < h.minLevel {
		return nil
	}
	var errs []error
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			err := handler.Handle(ctx, record.Clone())
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// WithAttrs adds the provide attributes to the handlers' attributes
func (h Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := []slog.Handler{}
	for _, handler := range h.handlers {
		newHandler := handler.WithAttrs(attrs)
		newHandlers = append(newHandlers, newHandler)
	}
	return Handler{handlers: newHandlers, minLevel: h.minLevel}
}

// WithGroup adds the provided group name to the handlers' groups
func (h Handler) WithGroup(name string) slog.Handler {
	newHandlers := []slog.Handler{}
	for _, handler := range h.handlers {
		newHandler := handler.WithGroup(name)
		newHandlers = append(newHandlers, newHandler)
	}
	return Handler{handlers: newHandlers, minLevel: h.minLevel}
}

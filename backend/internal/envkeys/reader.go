package envkeys

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Reader reads environment variables while accumulating any problems, so a
// single load can report every missing/malformed var at once rather than
// failing one at a time across repeated restarts. The zero value is ready to
// use; call Err after reading to get the accumulated result.
type Reader struct {
	errs []error
}

// Require reads a mandatory variable, recording an error when it is unset.
func (r *Reader) Require(key string) string {
	v := os.Getenv(key)
	if v == "" {
		r.errs = append(r.errs, fmt.Errorf("%s is required", key))
	}
	return v
}

// Optional reads a variable, falling back to fallback when it is unset.
func (r *Reader) Optional(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// DurationOr parses a duration variable, falling back to fallback when unset
// and recording an error when set but malformed.
func (r *Reader) DurationOr(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s: %w", key, err))
		return fallback
	}
	return d
}

// PositiveDurationOr parses a duration and validates it is positive, falling back
// to fallback when unset and recording an error when validation fails
func (r *Reader) PositiveDurationOr(key string, fallback time.Duration) time.Duration {
	d := r.DurationOr(key, fallback)
	if d <= 0 {
		r.errs = append(r.errs, fmt.Errorf("%s is not a positive duration: %s", key, d))
		return fallback
	}
	return d
}

// RequireBool parses a mandatory boolean variable, recording an error when it
// is unset or malformed.
func (r *Reader) RequireBool(key string) bool {
	b, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s: %w", key, err))
	}
	return b
}

// Err returns the accumulated read errors joined into one, or nil if there
// were none.
func (r *Reader) Err() error {
	return errors.Join(r.errs...)
}

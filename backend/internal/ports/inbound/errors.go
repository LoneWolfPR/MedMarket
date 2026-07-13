package inbound

import "errors"

//nolint:revive // sentinel errors are self-documenting
var (
	ErrValidation = errors.New("validation failed")
)

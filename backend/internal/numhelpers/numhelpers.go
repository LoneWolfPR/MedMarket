// Package numhelpers is a collection of helper functions
// for working with numbers
package numhelpers

import (
	"errors"
	"strconv"
)

//nolint:revive // sentinel errors are self-documenting
var ErrInvalidNumberString = errors.New("invalid number string")

// NormalizeNumberString takes a string, validates that it is a number
// Then removes any unnecessary trailing zeros and decimals
func NormalizeNumberString(numString string) (string, error) {
	numVal, err := strconv.ParseFloat(numString, 64)
	if err != nil {
		return "", ErrInvalidNumberString
	}

	return strconv.FormatFloat(numVal, 'f', -1, 64), nil
}

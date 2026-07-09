package shared

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Money is a universal type for storing a price or payment because
// different apis may use different formats for money
type Money struct {
	value int64
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrInvalidMoneyValue   = errors.New("invalid money value")
	ErrInvalidDollarString = errors.New("invalid dollar string")
)

var validDollarStringFmt = regexp.MustCompile(`^\d+(\.\d{2})?$`)

// NewMoneyFromCents is the constructor to set up the Money VO
func NewMoneyFromCents(cents int64) (Money, error) {
	if cents < 0 {
		return Money{}, ErrInvalidMoneyValue
	}
	return Money{value: cents}, nil
}

// NewMoneyFromDollars takes a dollar string, validates its format and constructs
// a Money VO from it
func NewMoneyFromDollars(dollars string) (Money, error) {
	if !validDollarStringFmt.MatchString(dollars) {
		return Money{}, ErrInvalidDollarString
	}

	centsStr := ""
	if strings.Contains(dollars, ".") {
		centsStr = strings.Replace(dollars, ".", "", 1)
	} else {
		centsStr = dollars + "00"
	}
	cents, err := strconv.ParseInt(centsStr, 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("error converting dollar string: %w", err)
	}
	return Money{value: cents}, nil
}

// IsZero checks if the value of money is zero
func (m Money) IsZero() bool { return m.value == 0 }

// Cents is the getter for the value
func (m Money) Cents() int64 { return m.value }

func (m Money) String() string {
	dollarVal := m.value / 100
	centsVal := m.value % 100
	return fmt.Sprintf("$%d.%02d", dollarVal, centsVal)
}

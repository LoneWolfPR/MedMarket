package shared

import (
	"errors"
	"fmt"
)

// Money is a universal type for storing a price or payment because
// different apis may use different formats for money
type Money struct {
	value int64
}

// ErrInvalidMoneyValue is a sentinel
var ErrInvalidMoneyValue = errors.New("invalid money value")

// NewMoneyFromCents is the constructor to set up the Money VO
func NewMoneyFromCents(cents int64) (Money, error) {
	if cents < 0 {
		return Money{}, ErrInvalidMoneyValue
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

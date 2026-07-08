package pharmacy

import (
	"errors"
	"unicode"
)

// NPI is the VO for storing a pharmacy's
// National Provider Identifier
type NPI struct {
	value string
}

// DEA is the VO for storing a pharmacy's
// Drug Enforcement Administration Number
type DEA struct {
	value string
}

// NCPDP is the VO for storing a pharmacy's
// National Council for Prescription Drug Programs Number
type NCPDP struct {
	value string
}

//nolint:revive // sentinel errors are self-documenting
var (
	ErrInvalidNPI   = errors.New("invalid NPI")
	ErrInvalidDEA   = errors.New("invalid DEA number")
	ErrInvalidNCPDP = errors.New("invalid NCPDP number")
)

// NewNPI is the constructor for setting up a new NPI
func NewNPI(npiNum string) (NPI, error) {
	if !isValidNPI(npiNum) {
		return NPI{}, ErrInvalidNPI
	}
	return NPI{value: npiNum}, nil
}

// NewDEA is the constructor for setting up a new DEA
func NewDEA(deaNum string) (DEA, error) {
	if !isValidDEA(deaNum) {
		return DEA{}, ErrInvalidDEA
	}
	return DEA{value: deaNum}, nil
}

// NewNCPDP is the constructor for setting up a new NCPDP
func NewNCPDP(ncpdpNum string) (NCPDP, error) {
	if !isValidNCPDP(ncpdpNum) {
		return NCPDP{}, ErrInvalidNCPDP
	}
	return NCPDP{value: ncpdpNum}, nil
}

// IsZero checks for an empty NPI value
func (npi NPI) IsZero() bool   { return npi.value == "" }
func (npi NPI) String() string { return npi.value }

// IsZero checks for an empty DEA value
func (dea DEA) IsZero() bool   { return dea.value == "" }
func (dea DEA) String() string { return dea.value }

// IsZero checks for an empty NCPDP value
func (ncpdp NCPDP) IsZero() bool   { return ncpdp.value == "" }
func (ncpdp NCPDP) String() string { return ncpdp.value }

// isValidNPI checks a string to verify if it confirms to the luhn pattern
func isValidNPI(value string) bool {
	const (
		reqLen           = 10
		luhnPrefix       = "80840"
		checkValConstant = 9
	)
	// Check Length
	if len(value) != reqLen {
		return false
	}

	luhnRunes := []rune(luhnPrefix + value)
	checksum := 0
	for i := len(luhnRunes) - 1; i >= 0; i-- {
		currRune := luhnRunes[i]
		if !unicode.IsDigit(currRune) {
			return false
		}
		// rune subtraction converts from ascii to actual int value
		checkValue := int(currRune - '0')
		if i%2 != 0 {
			checkValue *= 2
			if checkValue > checkValConstant {
				checkValue -= checkValConstant
			}
		}
		checksum += checkValue
	}
	return checksum%10 == 0
}

func isValidDEA(value string) bool {
	const (
		reqLen     = 9
		splitIndex = 2
	)
	if len(value) != reqLen {
		return false
	}

	// split at second index to validate letters and numbers
	letters := value[:splitIndex]
	digits := value[splitIndex:]

	// validate letters
	for _, char := range letters {
		if !unicode.IsLetter(char) {
			return false
		}
	}

	checkSum := 0
	currDigit := 0
	for index, digit := range digits {
		if !unicode.IsDigit(digit) {
			return false
		}
		currDigit = int(digit - '0')

		// skip last digit
		if index == len(digits)-1 {
			break
		}
		if index%2 == 0 {
			checkSum += currDigit
		} else {
			checkSum += currDigit * 2
		}
	}

	return checkSum%10 == currDigit
}

func isValidNCPDP(value string) bool {
	const reqLen = 7

	if len(value) != reqLen {
		return false
	}

	for _, digit := range value {
		if !unicode.IsDigit(digit) {
			return false
		}
	}

	return true
}

// Package shared contains types and functionality to be shared across all domains
package shared

// Address contains all the information in a street address
type Address struct {
	Street1 string
	Street2 string
	City    string
	State   string // 2 letter abbreviation
	Zip     string
}

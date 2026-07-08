package shared

// Address contains all the information in a street address
type Address struct {
	Street1 string
	Street2 string
	City    string
	State   string // 2 letter abbreviation
	Zip     string
}

// IsValid checks if the necessary address fields are set
func (a Address) IsValid() bool {
	return a.Street1 != "" &&
		a.City != "" &&
		a.State != "" &&
		a.Zip != ""
}

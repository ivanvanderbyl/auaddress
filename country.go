package anzaddress

// Country identifies the country whose postal conventions produced an address.
type Country string

const (
	// CountryAU identifies Australia.
	CountryAU Country = "AU"
	// CountryNZ identifies New Zealand.
	CountryNZ Country = "NZ"
)

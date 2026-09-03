package anzaddress

import (
	"errors"
	"strings"
)

var (
	ErrNoPostcode     = errors.New("no valid postcode found")
	ErrNoState        = errors.New("no valid state found")
	ErrNoDeliveryLine = errors.New("no delivery line found")
	ErrInvalidAddress = errors.New("invalid address format")
	ErrEmptyAddress   = errors.New("empty address")
)

// DeliveryPointKind identifies a street or postal delivery point.
type DeliveryPointKind uint8

const (
	// DeliveryPointStreet identifies a physical street delivery point.
	DeliveryPointStreet DeliveryPointKind = iota + 1
	// DeliveryPointPostal identifies a PO box, bag, or related postal delivery point.
	DeliveryPointPostal
)

// StreetDelivery contains the canonical components of a street delivery point.
type StreetDelivery struct {
	Unit         string
	Level        string
	StreetNumber string
	StreetName   string
	StreetType   string
	StreetSuffix string
}

// PostalDelivery contains the canonical type and identifier of a postal delivery point.
type PostalDelivery struct {
	Type   string
	Number string
}

// DeliveryPoint preserves one parsed delivery point and its kind.
type DeliveryPoint struct {
	Kind   DeliveryPointKind
	Street StreetDelivery
	Postal PostalDelivery
}

// ParsedAddress contains canonical delivery points, country-specific locality details, and compatibility fields.
type ParsedAddress struct {
	Country Country

	RawLines []string

	DeliveryPoints []DeliveryPoint

	IsPoBox     bool
	PoBoxType   string
	PoBoxNumber string

	Unit         string
	Level        string
	StreetNumber string
	StreetName   string
	StreetType   string
	StreetSuffix string

	BuildingName string

	Locality string
	State    string
	Postcode string

	NameLines []string
	Errors    []error
}

// Parser parses Australian and New Zealand address strings.
type Parser struct {
	strict bool
}

// Option configures a Parser.
type Option func(*Parser)

// WithStrict makes parsing return the first grammar or validation error directly.
func WithStrict(strict bool) Option {
	return func(p *Parser) {
		p.strict = strict
	}
}

// NewParser constructs a parser with the supplied options.
func NewParser(opts ...Option) *Parser {
	p := &Parser{
		strict: false,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Parse returns the first parsed address in raw.
func Parse(raw string) (*ParsedAddress, error) {
	return NewParser().Parse(raw)
}

// ParseAll returns every independently locality-terminated address in raw.
func ParseAll(raw string) ([]*ParsedAddress, error) {
	return NewParser().ParseAll(raw)
}

// Parse returns the first parsed address in raw using the parser's options.
func (p *Parser) Parse(raw string) (*ParsedAddress, error) {
	addresses, err := p.ParseAll(raw)
	if len(addresses) > 0 {
		return addresses[0], err
	}
	return &ParsedAddress{Errors: make([]error, 0)}, err
}

// ParseAll returns every independently locality-terminated address using the parser's options.
func (p *Parser) ParseAll(raw string) ([]*ParsedAddress, error) {
	normalised := normalise(raw)
	if normalised == "" {
		return nil, ErrEmptyAddress
	}

	lines := splitLines(normalised)
	if len(lines) == 0 {
		return nil, ErrEmptyAddress
	}

	tokens, err := lexAddress(normalised)
	if err == nil {
		if addresses, ok := parseNZAddressSequence(tokens, normalised); ok {
			return addresses, nil
		}
		var addresses []*ParsedAddress
		addresses, err = parseAddressSequence(tokens, normalised)
		if err == nil || p.strict {
			return addresses, err
		}
		if len(addresses) == 0 {
			addr := &ParsedAddress{RawLines: lines, Errors: []error{err}}
			return []*ParsedAddress{addr}, nil
		}
		last := addresses[len(addresses)-1]
		if len(last.Errors) == 0 || !errors.Is(last.Errors[len(last.Errors)-1], err) {
			last.Errors = append(last.Errors, err)
		}
		return addresses, nil
	}
	if p.strict {
		return nil, err
	}
	return []*ParsedAddress{{RawLines: lines, Errors: []error{err}}}, nil
}

func normalise(raw string) string {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	s = collapseSpaces(s)

	s = strings.TrimSpace(s)

	return s
}

func collapseSpaces(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	spacePending := false
	for _, ch := range s {
		if ch == ' ' || ch == '\t' {
			spacePending = true
			continue
		}
		if spacePending && result.Len() > 0 && ch != '\n' {
			result.WriteByte(' ')
		}
		spacePending = false
		result.WriteRune(ch)
	}
	return result.String()
}

func splitLines(s string) []string {
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = trimPunctuation(line)
		if line != "" {
			result = append(result, line)
		}
	}

	return result
}

func trimPunctuation(s string) string {
	s = strings.TrimLeft(s, ".,;:-–—")
	s = strings.TrimRight(s, ".,;:-–—")
	return strings.TrimSpace(s)
}

func (a *ParsedAddress) Format() string {
	var lines []string

	for _, name := range a.NameLines {
		lines = append(lines, strings.ToUpper(name))
	}

	lines = append(lines, a.FormatDeliveryLines()...)

	localityLine := a.FormatLocalityLine()
	if localityLine != "" {
		lines = append(lines, localityLine)
	}

	return strings.Join(lines, "\n")
}

func (a *ParsedAddress) FormatDeliveryLine() string {
	deliveryLines := a.FormatDeliveryLines()
	if len(deliveryLines) == 0 {
		return ""
	}

	return deliveryLines[0]
}

// FormatDeliveryLines formats every delivery point in encounter order.
func (a *ParsedAddress) FormatDeliveryLines() []string {
	if len(a.DeliveryPoints) == 0 {
		if line := a.formatLegacyDeliveryLine(); line != "" {
			return []string{line}
		}
		return nil
	}

	lines := make([]string, 0, len(a.DeliveryPoints))
	for _, point := range a.DeliveryPoints {
		var line string
		switch point.Kind {
		case DeliveryPointStreet:
			line = formatStreetDelivery(point.Street)
		case DeliveryPointPostal:
			line = formatPostalDelivery(point.Postal)
		}
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}

func (a *ParsedAddress) formatLegacyDeliveryLine() string {
	if a.IsPoBox {
		return formatPostalDelivery(PostalDelivery{
			Type:   a.PoBoxType,
			Number: a.PoBoxNumber,
		})
	}

	return formatStreetDelivery(StreetDelivery{
		Unit:         a.Unit,
		Level:        a.Level,
		StreetNumber: a.StreetNumber,
		StreetName:   a.StreetName,
		StreetType:   a.StreetType,
		StreetSuffix: a.StreetSuffix,
	})
}

func formatPostalDelivery(delivery PostalDelivery) string {
	return strings.TrimSpace(strings.Join([]string{delivery.Type, delivery.Number}, " "))
}

func formatStreetDelivery(delivery StreetDelivery) string {
	var parts []string

	if delivery.Unit != "" {
		parts = append(parts, delivery.Unit)
	}

	if delivery.Level != "" {
		parts = append(parts, delivery.Level)
	}

	if delivery.StreetNumber != "" {
		parts = append(parts, delivery.StreetNumber)
	}

	if delivery.StreetName != "" {
		parts = append(parts, delivery.StreetName)
	}

	if delivery.StreetType != "" {
		parts = append(parts, delivery.StreetType)
	}

	if delivery.StreetSuffix != "" {
		parts = append(parts, delivery.StreetSuffix)
	}

	return strings.Join(parts, " ")
}

func (a *ParsedAddress) FormatLocalityLine() string {
	if a.Locality == "" && a.State == "" && a.Postcode == "" {
		return ""
	}

	parts := []string{}
	if a.Locality != "" {
		parts = append(parts, a.Locality)
	}
	if a.State != "" {
		parts = append(parts, a.State)
	}
	if a.Postcode != "" {
		parts = append(parts, a.Postcode)
	}

	return strings.Join(parts, " ")
}

func (a *ParsedAddress) IsValid() bool {
	return len(a.Errors) == 0 && (a.Country == CountryAU || a.Country == CountryNZ) && a.Locality != "" && a.HasDeliveryPoint()
}

func (a *ParsedAddress) HasDeliveryPoint() bool {
	return len(a.DeliveryPoints) > 0 || a.IsPoBox || a.StreetNumber != "" || a.StreetName != ""
}

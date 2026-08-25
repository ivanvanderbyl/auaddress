package auaddress

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

type DeliveryPointKind uint8

const (
	DeliveryPointStreet DeliveryPointKind = iota + 1
	DeliveryPointPostal
)

type StreetDelivery struct {
	Unit         string
	Level        string
	StreetNumber string
	StreetName   string
	StreetType   string
	StreetSuffix string
}

type PostalDelivery struct {
	Type   string
	Number string
}

type DeliveryPoint struct {
	Kind   DeliveryPointKind
	Street StreetDelivery
	Postal PostalDelivery
}

type ParsedAddress struct {
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

type Parser struct {
	strict bool
}

type Option func(*Parser)

func WithStrict(strict bool) Option {
	return func(p *Parser) {
		p.strict = strict
	}
}

func NewParser(opts ...Option) *Parser {
	p := &Parser{
		strict: false,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func Parse(raw string) (*ParsedAddress, error) {
	return NewParser().Parse(raw)
}

func (p *Parser) Parse(raw string) (*ParsedAddress, error) {
	addr := &ParsedAddress{
		Errors: make([]error, 0),
	}

	normalised := normalise(raw)
	if normalised == "" {
		return addr, ErrEmptyAddress
	}

	lines := splitLines(normalised)
	if len(lines) == 0 {
		return addr, ErrEmptyAddress
	}

	addr.RawLines = lines

	tokens, err := lexAddress(normalised)
	if err == nil {
		err = parseAddressTokens(addr, tokens, normalised)
	}
	if err != nil {
		if p.strict {
			return addr, err
		}
		addr.Errors = append(addr.Errors, err)
	}

	return addr, nil
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
	return len(a.Errors) == 0 && a.Postcode != "" && a.State != ""
}

func (a *ParsedAddress) HasDeliveryPoint() bool {
	return a.IsPoBox || a.StreetNumber != "" || a.StreetName != ""
}

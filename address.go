package auaddress

import (
	"errors"
	"regexp"
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

	if err := p.parseLastLine(addr, lines); err != nil {
		if p.strict {
			return addr, err
		}
		addr.Errors = append(addr.Errors, err)
	}

	if len(lines) >= 2 {
		p.parseDeliveryLine(addr, lines[len(lines)-2])
	} else if p.strict {
		return addr, ErrNoDeliveryLine
	} else {
		addr.Errors = append(addr.Errors, ErrNoDeliveryLine)
	}

	if len(lines) >= 3 {
		addr.NameLines = lines[:len(lines)-2]
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

var multiSpaceRegex = regexp.MustCompile(`[ \t]+`)

func collapseSpaces(s string) string {
	return multiSpaceRegex.ReplaceAllString(s, " ")
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

var postcodeRegex = regexp.MustCompile(`^\d{4}$`)

func (p *Parser) parseLastLine(addr *ParsedAddress, lines []string) error {
	if len(lines) == 0 {
		return ErrInvalidAddress
	}

	lastLine := strings.ToUpper(lines[len(lines)-1])
	tokens := strings.Fields(lastLine)

	if len(tokens) < 3 {
		return ErrInvalidAddress
	}

	postcode := tokens[len(tokens)-1]
	if !postcodeRegex.MatchString(postcode) {
		return ErrNoPostcode
	}
	addr.Postcode = postcode

	state := tokens[len(tokens)-2]
	if _, ok := validStates[state]; !ok {
		return ErrNoState
	}
	addr.State = state

	localityTokens := tokens[:len(tokens)-2]
	addr.Locality = strings.Join(localityTokens, " ")

	return nil
}

func (p *Parser) parseDeliveryLine(addr *ParsedAddress, line string) {
	line = strings.TrimSpace(line)
	upperLine := strings.ToUpper(line)

	if p.parsePoBox(addr, upperLine) {
		return
	}

	p.parseStreetAddress(addr, line)
}

var poBoxPatterns = []struct {
	pattern *regexp.Regexp
	boxType string
}{
	{regexp.MustCompile(`(?i)^(LOCKED\s+BAG)\s+(.+)$`), "LOCKED BAG"},
	{regexp.MustCompile(`(?i)^(PRIVATE\s+BAG)\s+(.+)$`), "PRIVATE BAG"},
	{regexp.MustCompile(`(?i)^(GPO\s+BOX)\s+(.+)$`), "GPO BOX"},
	{regexp.MustCompile(`(?i)^(G\.?P\.?O\.?\s*BOX)\s+(.+)$`), "GPO BOX"},
	{regexp.MustCompile(`(?i)^(PO\s+BOX)\s+(.+)$`), "PO BOX"},
	{regexp.MustCompile(`(?i)^(P\.?O\.?\s*BOX)\s+(.+)$`), "PO BOX"},
	{regexp.MustCompile(`(?i)^(REPLY\s+PAID)\s+(.+)$`), "REPLY PAID"},
	{regexp.MustCompile(`(?i)^(RMB)\s+(.+)$`), "RMB"},
	{regexp.MustCompile(`(?i)^(CMB)\s+(.+)$`), "CMB"},
	{regexp.MustCompile(`(?i)^(RSD)\s+(.+)$`), "RSD"},
	{regexp.MustCompile(`(?i)^(MS)\s+(.+)$`), "MS"},
	{regexp.MustCompile(`(?i)^(CMA)\s+(.+)$`), "CMA"},
	{regexp.MustCompile(`(?i)^(CPA)\s+(.+)$`), "CPA"},
	{regexp.MustCompile(`(?i)^(CARE\s+PO)\s+(.+)$`), "CARE PO"},
}

func (p *Parser) parsePoBox(addr *ParsedAddress, line string) bool {
	for _, pattern := range poBoxPatterns {
		matches := pattern.pattern.FindStringSubmatch(line)
		if matches != nil {
			addr.IsPoBox = true
			addr.PoBoxType = pattern.boxType
			addr.PoBoxNumber = strings.TrimSpace(matches[2])
			return true
		}
	}
	return false
}

var (
	unitSlashNumberRegex = regexp.MustCompile(`^(\d+[A-Za-z]?)\s*/\s*(\d+[A-Za-z]?(?:\s*-\s*\d+[A-Za-z]?)?)(.*)$`)
	unitPrefixRegex      = regexp.MustCompile(`(?i)^(UNIT|FLAT|APT|APARTMENT|VILLA|LOT|SHOP|SH|SUITE|STE|ROOM|RM|OFFICE|OFF|FACTORY|FY|WAREHOUSE|WE|SHED|SD|KIOSK|KSK|TOWNHOUSE|TNHS|PENTHOUSE|PTHS)\s+(\d+[A-Za-z]?)\s*[,]?\s*(.*)$`)
	levelPrefixRegex     = regexp.MustCompile(`(?i)^(LEVEL|LVL|L|FLOOR|FL)\s+(\d+[A-Za-z]?|G|B|M|P|LG|UG)\s*[,]?\s*(.*)$`)
	streetNumberRegex    = regexp.MustCompile(`^(\d+[A-Za-z]?(?:\s*-\s*\d+[A-Za-z]?)?)\s+(.+)$`)
)

func (p *Parser) parseStreetAddress(addr *ParsedAddress, line string) {
	remaining := line

	if matches := unitSlashNumberRegex.FindStringSubmatch(remaining); matches != nil {
		addr.Unit = strings.ToUpper(matches[1])
		addr.StreetNumber = strings.ToUpper(strings.ReplaceAll(matches[2], " ", ""))
		remaining = strings.TrimSpace(matches[3])
	} else {
		if matches := unitPrefixRegex.FindStringSubmatch(remaining); matches != nil {
			unitType := strings.ToUpper(matches[1])
			if normalised, ok := unitTypes[unitType]; ok {
				addr.Unit = normalised + " " + strings.ToUpper(matches[2])
			} else {
				addr.Unit = unitType + " " + strings.ToUpper(matches[2])
			}
			remaining = strings.TrimSpace(matches[3])
		}

		if matches := levelPrefixRegex.FindStringSubmatch(remaining); matches != nil {
			levelType := strings.ToUpper(matches[1])
			if normalised, ok := levelTypes[levelType]; ok {
				addr.Level = normalised + " " + strings.ToUpper(matches[2])
			} else {
				addr.Level = levelType + " " + strings.ToUpper(matches[2])
			}
			remaining = strings.TrimSpace(matches[3])
		}
	}

	if addr.StreetNumber == "" {
		if matches := streetNumberRegex.FindStringSubmatch(remaining); matches != nil {
			addr.StreetNumber = strings.ToUpper(strings.ReplaceAll(matches[1], " ", ""))
			remaining = strings.TrimSpace(matches[2])
		}
	}

	p.parseStreetNameAndType(addr, remaining)
}

func (p *Parser) parseStreetNameAndType(addr *ParsedAddress, s string) {
	tokens := strings.Fields(strings.ToUpper(s))
	if len(tokens) == 0 {
		return
	}

	if len(tokens) >= 2 {
		lastToken := tokens[len(tokens)-1]
		if _, ok := streetSuffixes[lastToken]; ok {
			if normalised, ok := streetSuffixes[lastToken]; ok {
				addr.StreetSuffix = normalised
			}
			tokens = tokens[:len(tokens)-1]
		}
	}

	if len(tokens) >= 1 {
		lastToken := tokens[len(tokens)-1]
		if normalised, ok := streetTypes[lastToken]; ok {
			addr.StreetType = normalised
			tokens = tokens[:len(tokens)-1]
		}
	}

	if len(tokens) > 0 {
		addr.StreetName = strings.Join(tokens, " ")
	}
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

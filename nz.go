package anzaddress

import "strings"

var nzRegionKeywords = newKeywordTable(map[string]string{
	"AUCKLAND":           "AUCKLAND",
	"BAY OF PLENTY":      "BAY OF PLENTY",
	"CANTERBURY":         "CANTERBURY",
	"GISBORNE":           "GISBORNE",
	"HAWKE'S BAY":        "HAWKE'S BAY",
	"HAWKES BAY":         "HAWKE'S BAY",
	"MANAWATU-WANGANUI":  "MANAWATŪ-WHANGANUI",
	"MANAWATŪ-WHANGANUI": "MANAWATŪ-WHANGANUI",
	"MARLBOROUGH":        "MARLBOROUGH",
	"NELSON":             "NELSON",
	"NORTHLAND":          "NORTHLAND",
	"OTAGO":              "OTAGO",
	"SOUTHLAND":          "SOUTHLAND",
	"TARANAKI":           "TARANAKI",
	"TASMAN":             "TASMAN",
	"WAIKATO":            "WAIKATO",
	"WELLINGTON":         "WELLINGTON",
	"WEST COAST":         "WEST COAST",
})

var nzCountryKeywords = newKeywordTable(map[string]string{
	"NEW ZEALAND": "NZ",
	"NZ":          "NZ",
	"NZL":         "NZ",
})

type nzLocalityMatch struct {
	name string
	next int
}

func parseNZAddressSequence(tokens []token, normalized string) ([]*ParsedAddress, bool) {
	for deliveryStart := skipSoftTokens(tokens, 0); deliveryStart < len(tokens); deliveryStart++ {
		deliveryStart = skipSoftTokens(tokens, deliveryStart)
		if isEOF(tokens, deliveryStart) || !canStartDelivery(tokens, deliveryStart) {
			continue
		}

		for localityStart := deliveryStart + 1; localityStart < len(tokens); localityStart++ {
			localityStart = skipSoftTokens(tokens, localityStart)
			if isEOF(tokens, localityStart) {
				break
			}
			locality, ok := matchNZLocality(tokens, localityStart)
			if !ok {
				continue
			}

			points, next, deliveryOK := recognizeDeliverySequence(tokens, deliveryStart, localityStart)
			if !deliveryOK || skipSoftTokensBefore(tokens, next, localityStart) != localityStart {
				continue
			}
			if !hasNZDeliveryType(points) {
				continue
			}

			region, postcode, countryMarked, end := parseNZTail(tokens, locality.next)
			if region == "" && !countryMarked {
				continue
			}
			if !isEOF(tokens, skipSoftTokens(tokens, end)) {
				continue
			}

			address := &ParsedAddress{
				Country:        CountryNZ,
				DeliveryPoints: points,
				Locality:       locality.name,
				State:          region,
				Postcode:       postcode,
				Errors:         make([]error, 0),
			}
			projectLegacyDeliveryFields(address)
			address.RawLines = splitLines(normalized[tokens[deliveryStart].offset:])
			return []*ParsedAddress{address}, true
		}
	}
	return nil, false
}

func hasNZDeliveryType(points []DeliveryPoint) bool {
	for _, point := range points {
		if point.Kind == DeliveryPointPostal || point.Street.StreetType != "" {
			return true
		}
	}
	return false
}

func matchNZLocality(tokens []token, start int) (nzLocalityMatch, bool) {
	position := skipSoftTokens(tokens, start)
	parts := make([]string, 0, nzMaxLocalityTokens)
	best := nzLocalityMatch{}
	found := false

	for position < len(tokens) && len(parts) < nzMaxLocalityTokens {
		current := tokens[position]
		if current.kind != tokenWord && current.kind != tokenNumberish {
			break
		}
		parts = append(parts, current.value)
		position++

		name := strings.Join(parts, " ")
		if _, ok := nzLocalities[name]; ok {
			best = nzLocalityMatch{name: name, next: position}
			found = true
		}
		position = skipSoftTokens(tokens, position)
	}
	return best, found
}

func parseNZTail(tokens []token, start int) (region, postcode string, countryMarked bool, end int) {
	position := skipSoftTokens(tokens, start)
	if matchedRegion, next, ok := matchKeyword(tokens, position, len(tokens), nzRegionKeywords); ok {
		region = matchedRegion
		position = skipSoftTokens(tokens, next)
	}
	if !isEOF(tokens, position) && isPostcode(tokens[position].value) {
		postcode = tokens[position].value
		position = skipSoftTokens(tokens, position+1)
	}
	if next, ok := matchNZCountry(tokens, position); ok {
		countryMarked = true
		position = skipSoftTokens(tokens, next)
	}
	return region, postcode, countryMarked, position
}

func matchNZCountry(tokens []token, start int) (int, bool) {
	value, next, ok := matchKeyword(tokens, start, len(tokens), nzCountryKeywords)
	return next, ok && value == "NZ"
}

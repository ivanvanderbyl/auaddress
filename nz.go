package anzaddress

import "strings"

var nzRegionKeywords = newKeywordTable(map[string]string{
	"AUCKLAND":           "AUCKLAND",
	"BAY OF PLENTY":      "BAY OF PLENTY",
	"CANTERBURY":         "CANTERBURY",
	"CHATHAM ISLANDS":    "CHATHAM ISLANDS",
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

type nzAddressTail struct {
	start    int
	locality nzLocalityMatch
	region   string
	postcode string
}

type nzAddressSegment struct {
	deliveryStart int
	points        []DeliveryPoint
	tail          nzAddressTail
	end           int
}

func parseNZAddressSequence(tokens []token, normalized string) ([]*ParsedAddress, bool) {
	position := skipSoftTokens(tokens, 0)
	first, ok := findFirstNZAddressSegment(tokens, position)
	if !ok {
		return nil, false
	}

	sharedNames := []string(nil)
	if first.deliveryStart < len(tokens) && tokens[first.deliveryStart].offset > 0 {
		sharedNames = splitLines(normalized[:tokens[first.deliveryStart].offset])
	}

	addresses := make([]*ParsedAddress, 0, 2)
	segment := first
	for {
		addresses = append(addresses, nzAddressFromSegment(segment, tokens, normalized, sharedNames))

		position = skipSoftTokens(tokens, segment.end)
		if isEOF(tokens, position) {
			return addresses, true
		}

		var segmentOK bool
		segment, segmentOK = parseNZAddressSegmentAt(tokens, position)
		if !segmentOK {
			return nil, false
		}
	}
}

func findFirstNZAddressSegment(tokens []token, start int) (nzAddressSegment, bool) {
	for position := start; position < len(tokens); position++ {
		position = skipSoftTokens(tokens, position)
		if isEOF(tokens, position) {
			break
		}
		if !canStartDelivery(tokens, position) {
			continue
		}
		if segment, ok := parseNZAddressSegmentAt(tokens, position); ok {
			return segment, true
		}
	}
	return nzAddressSegment{}, false
}

func parseNZAddressSegmentAt(tokens []token, deliveryStart int) (nzAddressSegment, bool) {
	deliveryStart = skipSoftTokens(tokens, deliveryStart)
	var best nzAddressSegment
	found := false

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
		if !deliveryOK || skipSoftTokensBefore(tokens, next, localityStart) != localityStart || !hasNZDeliveryType(points) {
			continue
		}

		region, postcode, countryMarked, end := parseNZTail(tokens, locality.next)
		if region == "" && !countryMarked {
			continue
		}

		segment := nzAddressSegment{
			deliveryStart: deliveryStart,
			points:        points,
			tail: nzAddressTail{
				start:    localityStart,
				locality: locality,
				region:   region,
				postcode: postcode,
			},
			end: end,
		}
		if !nzSequenceTailCanEnd(tokens, segment) {
			continue
		}
		if !found || preferNZLocalitySegment(segment, best, tokens) {
			best = segment
			found = true
		}
	}

	return best, found
}

func preferNZLocalitySegment(candidate, current nzAddressSegment, tokens []token) bool {
	if candidate.end != current.end {
		return candidate.end < current.end
	}
	candidateBoundary := hasExplicitBoundaryBefore(tokens, candidate.tail.start)
	currentBoundary := hasExplicitBoundaryBefore(tokens, current.tail.start)
	if candidateBoundary != currentBoundary {
		return candidateBoundary
	}
	return candidate.tail.start < current.tail.start
}

func nzSequenceTailCanEnd(tokens []token, segment nzAddressSegment) bool {
	position := skipSoftTokens(tokens, segment.end)
	return isEOF(tokens, position) || canStartDelivery(tokens, position)
}

func nzAddressFromSegment(segment nzAddressSegment, tokens []token, normalized string, sharedNames []string) *ParsedAddress {
	address := &ParsedAddress{
		Country:        CountryNZ,
		DeliveryPoints: segment.points,
		Locality:       segment.tail.locality.name,
		State:          segment.tail.region,
		Postcode:       segment.tail.postcode,
		NameLines:      append([]string(nil), sharedNames...),
		Errors:         make([]error, 0),
	}
	projectLegacyDeliveryFields(address)

	startOffset := tokens[segment.deliveryStart].offset
	endOffset := len(normalized)
	if segment.end < len(tokens) && tokens[segment.end].kind != tokenEOF {
		endOffset = tokens[segment.end].offset
	}
	address.RawLines = append(append([]string(nil), sharedNames...), splitLines(normalized[startOffset:endOffset])...)
	return address
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

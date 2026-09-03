package anzaddress

type addressTail struct {
	start    int
	locality localityMatch
	state    string
	postcode string
	err      error
}

type addressSegment struct {
	deliveryStart int
	points        []DeliveryPoint
	tail          addressTail
	end           int
}

func parseAddressSequence(tokens []token, normalized string) ([]*ParsedAddress, error) {
	position := skipSoftTokens(tokens, 0)
	first, ok := findFirstAddressSegment(tokens, position)
	if !ok {
		addr := &ParsedAddress{
			RawLines: splitLines(normalized),
			Errors:   make([]error, 0),
		}
		return []*ParsedAddress{addr}, classifyUnparsedAddress(tokens)
	}

	sharedNames := []string(nil)
	if first.deliveryStart < len(tokens) && tokens[first.deliveryStart].offset > 0 {
		sharedNames = splitLines(normalized[:tokens[first.deliveryStart].offset])
	}

	addresses := make([]*ParsedAddress, 0, 2)
	segment := first
	for {
		addr := addressFromSegment(segment, tokens, normalized, sharedNames)
		addresses = append(addresses, addr)
		if segment.tail.err != nil {
			addr.Errors = append(addr.Errors, segment.tail.err)
			return addresses, segment.tail.err
		}

		position = skipSoftTokens(tokens, segment.end)
		if isEOF(tokens, position) {
			return addresses, nil
		}

		var segmentOK bool
		segment, segmentOK = parseAddressSegmentAt(tokens, position)
		if !segmentOK {
			return addresses, ErrInvalidAddress
		}
	}
}

func findFirstAddressSegment(tokens []token, start int) (addressSegment, bool) {
	for position := start; position < len(tokens); position++ {
		position = skipSoftTokens(tokens, position)
		if isEOF(tokens, position) {
			break
		}
		if !canStartDelivery(tokens, position) {
			continue
		}
		if segment, ok := parseAddressSegmentAt(tokens, position); ok {
			return segment, true
		}
	}
	return addressSegment{}, false
}

func parseAddressSegmentAt(tokens []token, deliveryStart int) (addressSegment, bool) {
	deliveryStart = skipSoftTokens(tokens, deliveryStart)
	var bestValid addressSegment
	validFound := false
	var firstInvalid addressSegment
	invalidFound := false

	for position := deliveryStart + 1; position < len(tokens); position++ {
		localityStart := skipSoftTokens(tokens, position)
		if isEOF(tokens, localityStart) {
			break
		}
		if validFound && localityStart >= bestValid.end {
			break
		}

		locality, ok := matchLocality(tokens, localityStart)
		if !ok {
			position = localityStart
			continue
		}

		points, next, deliveryOK := recognizeDeliverySequence(tokens, deliveryStart, localityStart)
		if !deliveryOK || skipSoftTokensBefore(tokens, next, localityStart) != localityStart {
			position = localityStart
			continue
		}

		tail, end := parseSequenceTail(tokens, localityStart, locality)
		segment := addressSegment{
			deliveryStart: deliveryStart,
			points:        points,
			tail:          tail,
			end:           end,
		}
		if tail.err != nil {
			if !invalidFound {
				firstInvalid = segment
				invalidFound = true
			}
			if !isEOF(tokens, end) && canStartDelivery(tokens, end) {
				return segment, true
			}
			position = localityStart
			continue
		}
		if sequenceTailCanEnd(tokens, segment) {
			if !validFound || preferLocalitySegment(segment, bestValid, tokens) {
				bestValid = segment
				validFound = true
			}
		}
		position = localityStart
	}

	if validFound {
		return bestValid, true
	}
	return firstInvalid, invalidFound
}

func preferLocalitySegment(candidate, current addressSegment, tokens []token) bool {
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

func hasExplicitBoundaryBefore(tokens []token, position int) bool {
	return position > 0 && isSoftToken(tokens[position-1])
}

func parseSequenceTail(tokens []token, start int, locality localityMatch) (addressTail, int) {
	tail := addressTail{start: start, locality: locality}
	position := skipSoftTokens(tokens, locality.next)
	if isEOF(tokens, position) {
		return tail, position
	}

	current := tokens[position]
	if state, next, isState := matchKeyword(tokens, position, len(tokens), stateKeywords); isState {
		tail.state = state
		if !locality.states.contains(state) {
			tail.err = ErrNoState
		}
		position = skipSoftTokens(tokens, next)
	} else if current.kind == tokenWord && hasPostcodeAfter(tokens, position+1) {
		tail.err = ErrNoState
		position = skipSoftTokens(tokens, position+1)
	}

	if isEOF(tokens, position) {
		return tail, position
	}

	current = tokens[position]
	if isPostcode(current.value) {
		if postcodeStartsNextAddress(tokens, position) {
			return tail, position
		}
		tail.postcode = current.value
		return tail, skipSoftTokens(tokens, position+1)
	}
	if canStartDelivery(tokens, position) {
		return tail, position
	}
	if tail.state != "" {
		tail.err = ErrNoPostcode
	}
	return tail, position
}

func postcodeStartsNextAddress(tokens []token, position int) bool {
	afterPostcode := skipSoftTokens(tokens, position+1)
	if !isEOF(tokens, afterPostcode) {
		if remainder, ok := parseAddressSegmentAt(tokens, afterPostcode); ok && remainder.tail.err == nil {
			return false
		}
	}
	next, ok := parseAddressSegmentAt(tokens, position)
	return ok && next.tail.err == nil && len(next.points) == 1 && next.points[0].Kind == DeliveryPointStreet
}

func sequenceTailCanEnd(tokens []token, segment addressSegment) bool {
	position := skipSoftTokens(tokens, segment.end)
	return isEOF(tokens, position) || segment.tail.postcode != "" || canStartDelivery(tokens, position)
}

func canStartDelivery(tokens []token, position int) bool {
	position = skipSoftTokens(tokens, position)
	if isEOF(tokens, position) {
		return false
	}
	if _, _, ok := matchKeyword(tokens, position, len(tokens), postalKeywords); ok {
		return true
	}
	if _, _, ok := matchKeyword(tokens, position, len(tokens), unitKeywords); ok {
		return true
	}
	if _, _, ok := matchKeyword(tokens, position, len(tokens), levelKeywords); ok {
		return true
	}
	return tokens[position].kind == tokenNumberish
}

func classifyUnparsedAddress(tokens []token) error {
	for position := 0; position < len(tokens); position++ {
		position = skipSoftTokens(tokens, position)
		if isEOF(tokens, position) {
			break
		}
		if _, ok := matchLocality(tokens, position); ok {
			return ErrNoDeliveryLine
		}
	}
	return ErrInvalidAddress
}

func addressFromSegment(segment addressSegment, tokens []token, normalized string, sharedNames []string) *ParsedAddress {
	addr := &ParsedAddress{
		Country:        CountryAU,
		DeliveryPoints: segment.points,
		Locality:       segment.tail.locality.name,
		State:          segment.tail.state,
		Postcode:       segment.tail.postcode,
		NameLines:      append([]string(nil), sharedNames...),
		Errors:         make([]error, 0),
	}
	projectLegacyDeliveryFields(addr)

	startOffset := tokens[segment.deliveryStart].offset
	endOffset := len(normalized)
	if segment.end < len(tokens) && tokens[segment.end].kind != tokenEOF {
		endOffset = tokens[segment.end].offset
	}
	addr.RawLines = append(append([]string(nil), sharedNames...), splitLines(normalized[startOffset:endOffset])...)
	return addr
}

func projectLegacyDeliveryFields(addr *ParsedAddress) {
	for _, point := range addr.DeliveryPoints {
		switch point.Kind {
		case DeliveryPointStreet:
			if addr.StreetNumber == "" && addr.StreetName == "" {
				addr.Unit = point.Street.Unit
				addr.Level = point.Street.Level
				addr.StreetNumber = point.Street.StreetNumber
				addr.StreetName = point.Street.StreetName
				addr.StreetType = point.Street.StreetType
				addr.StreetSuffix = point.Street.StreetSuffix
			}
		case DeliveryPointPostal:
			addr.IsPoBox = true
			if addr.PoBoxType == "" {
				addr.PoBoxType = point.Postal.Type
				addr.PoBoxNumber = point.Postal.Number
			}
		}
	}
}

func hasPostcodeAfter(tokens []token, start int) bool {
	position := skipSoftTokens(tokens, start)
	return position < len(tokens) && isPostcode(tokens[position].value)
}

func isPostcode(value string) bool {
	if len(value) != 4 {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func isEOF(tokens []token, position int) bool {
	return position < len(tokens) && tokens[position].kind == tokenEOF
}

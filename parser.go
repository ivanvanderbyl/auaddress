package auaddress

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
	addresses := make([]*ParsedAddress, 0, 2)
	sharedNames := []string(nil)

	for !isEOF(tokens, position) {
		segment, ok := findAddressSegment(tokens, position, len(addresses) == 0)
		if !ok {
			if len(addresses) > 0 {
				return addresses, ErrInvalidAddress
			}

			addr := &ParsedAddress{RawLines: splitLines(normalized), Errors: make([]error, 0)}
			err := parseAddressTokens(addr, tokens, normalized)
			return []*ParsedAddress{addr}, err
		}

		if len(addresses) == 0 && segment.deliveryStart < len(tokens) && tokens[segment.deliveryStart].offset > 0 {
			sharedNames = splitLines(normalized[:tokens[segment.deliveryStart].offset])
		}

		addr := addressFromSegment(segment, tokens, normalized, sharedNames)
		addresses = append(addresses, addr)
		if segment.tail.err != nil {
			return addresses, segment.tail.err
		}
		position = skipSoftTokens(tokens, segment.end)
	}

	return addresses, nil
}

func findAddressSegment(tokens []token, start int, allowPrefix bool) (addressSegment, bool) {
	bestValid := addressSegment{}
	validFound := false
	bestInvalid := addressSegment{}
	invalidFound := false

	for position := start; position < len(tokens); position++ {
		localityStart := skipSoftTokens(tokens, position)
		if isEOF(tokens, localityStart) {
			break
		}

		locality, ok := matchLocality(tokens, localityStart)
		if !ok {
			position = localityStart
			continue
		}

		tail, end := parseSequenceTail(tokens, localityStart, locality)
		deliveryStart, points, deliveryOK := findDeliverySequenceFrom(tokens, start, localityStart, allowPrefix)
		if !deliveryOK {
			position = localityStart
			continue
		}

		segment := addressSegment{
			deliveryStart: deliveryStart,
			points:        points,
			tail:          tail,
			end:           end,
		}
		if tail.err != nil {
			if !invalidFound || preferSegment(segment, bestInvalid, tokens) {
				bestInvalid = segment
				invalidFound = true
			}
			position = localityStart
			continue
		}

		if !sequenceTailCanEnd(tokens, segment) {
			position = localityStart
			continue
		}
		if !validFound || preferSegment(segment, bestValid, tokens) {
			bestValid = segment
			validFound = true
		}
		position = localityStart
	}

	if validFound {
		return bestValid, true
	}
	return bestInvalid, invalidFound
}

func findDeliverySequenceFrom(tokens []token, start, limit int, allowPrefix bool) (int, []DeliveryPoint, bool) {
	if !allowPrefix {
		position := skipSoftTokensBefore(tokens, start, limit)
		points, next, ok := recognizeDeliverySequence(tokens, position, limit)
		if ok && skipSoftTokensBefore(tokens, next, limit) == limit {
			return position, points, true
		}
		return 0, nil, false
	}

	for position := start; position < limit; position++ {
		if isSoftToken(tokens[position]) {
			continue
		}
		points, next, ok := recognizeDeliverySequence(tokens, position, limit)
		if ok && skipSoftTokensBefore(tokens, next, limit) == limit {
			return position, points, true
		}
	}
	return 0, nil, false
}

func parseSequenceTail(tokens []token, start int, locality localityMatch) (addressTail, int) {
	tail := addressTail{start: start, locality: locality}
	position := skipSoftTokens(tokens, locality.next)
	if isEOF(tokens, position) {
		return tail, position
	}

	current := tokens[position]
	if _, isState := validStates[current.value]; isState {
		tail.state = current.value
		if !locality.states.contains(current.value) {
			tail.err = ErrNoState
		}
		position = skipSoftTokens(tokens, position+1)
	} else if current.kind == tokenWord && hasPostcodeAfter(tokens, position+1) {
		tail.err = ErrNoState
		position = skipSoftTokens(tokens, position+1)
	}

	if isEOF(tokens, position) {
		return tail, position
	}

	current = tokens[position]
	if isPostcode(current.value) {
		tail.postcode = current.value
		position = skipSoftTokens(tokens, position+1)
	} else if tail.state != "" {
		tail.err = ErrNoPostcode
	}

	return tail, position
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

func preferSegment(candidate, current addressSegment, tokens []token) bool {
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

func addressFromSegment(segment addressSegment, tokens []token, normalized string, sharedNames []string) *ParsedAddress {
	addr := &ParsedAddress{
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

func parseAddressTokens(addr *ParsedAddress, tokens []token, normalized string) error {
	tail, ok := findAddressTail(tokens)
	if !ok {
		return ErrInvalidAddress
	}

	addr.Locality = tail.locality.name
	addr.State = tail.state
	addr.Postcode = tail.postcode

	deliveryStart, points, ok := findDeliverySequence(tokens, tail.start)
	if !ok {
		return ErrNoDeliveryLine
	}
	addr.DeliveryPoints = points
	projectLegacyDeliveryFields(addr)

	if deliveryStart < len(tokens) && tokens[deliveryStart].offset > 0 {
		addr.NameLines = splitLines(normalized[:tokens[deliveryStart].offset])
	}

	return tail.err
}

func findAddressTail(tokens []token) (addressTail, bool) {
	bestInvalid := addressTail{start: -1}
	invalidFound := false
	bestValid := addressTail{start: -1}
	validFound := false
	bestBoundary := false

	for position := 0; position < len(tokens); position++ {
		start := skipSoftTokens(tokens, position)
		if start >= len(tokens) || tokens[start].kind == tokenEOF {
			break
		}

		locality, ok := matchLocality(tokens, start)
		if !ok {
			position = start
			continue
		}

		candidate := parseAddressTail(tokens, start, locality)
		if candidate.err == nil {
			boundary := hasExplicitBoundaryBefore(tokens, start)
			if !validFound || boundary && !bestBoundary {
				bestValid = candidate
				bestBoundary = boundary
				validFound = true
			}
			position = start
			continue
		}
		if !invalidFound || candidate.start > bestInvalid.start {
			bestInvalid = candidate
			invalidFound = true
		}
		position = start
	}

	if validFound {
		return bestValid, true
	}
	return bestInvalid, invalidFound
}

func hasExplicitBoundaryBefore(tokens []token, position int) bool {
	return position > 0 && isSoftToken(tokens[position-1])
}

func parseAddressTail(tokens []token, start int, locality localityMatch) addressTail {
	tail := addressTail{start: start, locality: locality}
	position := skipSoftTokens(tokens, locality.next)
	if isEOF(tokens, position) {
		return tail
	}

	current := tokens[position]
	if _, isState := validStates[current.value]; isState {
		tail.state = current.value
		if !locality.states.contains(current.value) {
			tail.err = ErrNoState
		}
		position = skipSoftTokens(tokens, position+1)
	} else if current.kind == tokenWord && hasPostcodeAfter(tokens, position+1) {
		tail.err = ErrNoState
		position = skipSoftTokens(tokens, position+1)
	}

	if isEOF(tokens, position) {
		return tail
	}

	current = tokens[position]
	if isPostcode(current.value) {
		tail.postcode = current.value
		position = skipSoftTokens(tokens, position+1)
	} else {
		if tail.state != "" {
			tail.err = ErrNoPostcode
		} else if tail.err == nil {
			tail.err = ErrInvalidAddress
		}
		return tail
	}

	if !isEOF(tokens, position) {
		tail.err = ErrInvalidAddress
	}
	return tail
}

func findDeliverySequence(tokens []token, limit int) (int, []DeliveryPoint, bool) {
	for start := 0; start < limit; start++ {
		if isSoftToken(tokens[start]) {
			continue
		}
		points, next, ok := recognizeDeliverySequence(tokens, start, limit)
		if ok && skipSoftTokensBefore(tokens, next, limit) == limit {
			return start, points, true
		}
	}
	return 0, nil, false
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

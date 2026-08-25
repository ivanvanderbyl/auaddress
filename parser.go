package auaddress

type addressTail struct {
	start    int
	locality localityMatch
	state    string
	postcode string
	err      error
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

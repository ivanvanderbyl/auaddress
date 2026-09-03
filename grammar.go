package anzaddress

import "strings"

type keywordTable struct {
	values   map[string]string
	maxWords int
}

var (
	postalKeywords = newKeywordTable(deliveryPointKeywords)
	unitKeywords   = newKeywordTable(unitTypes)
	levelKeywords  = newKeywordTable(levelTypes)
	stateKeywords  = newKeywordTable(stateTypes)
)

func newKeywordTable(source map[string]string) keywordTable {
	table := keywordTable{values: make(map[string]string, len(source))}
	for keyword, normalized := range source {
		keyword = normalizeAddressAtom(strings.Join(strings.Fields(keyword), " "))
		table.values[keyword] = normalized
		if words := len(strings.Fields(keyword)); words > table.maxWords {
			table.maxWords = words
		}
	}
	return table
}

func matchKeyword(tokens []token, start, limit int, table keywordTable) (string, int, bool) {
	position := skipSoftTokensBefore(tokens, start, limit)
	parts := make([]string, 0, table.maxWords)
	bestValue := ""
	bestNext := position

	for position < limit && len(parts) < table.maxWords {
		current := tokens[position]
		if current.kind != tokenWord && current.kind != tokenNumberish {
			break
		}

		parts = append(parts, current.value)
		position++
		if value, ok := table.values[strings.Join(parts, " ")]; ok {
			bestValue = value
			bestNext = position
		}
		position = skipSoftTokensBefore(tokens, position, limit)
	}

	return bestValue, bestNext, bestValue != ""
}

func recognizePostal(tokens []token, start, limit int) (DeliveryPoint, int, bool) {
	postalType, position, ok := matchKeyword(tokens, start, limit, postalKeywords)
	if !ok {
		return DeliveryPoint{}, start, false
	}

	position = skipSoftTokensBefore(tokens, position, limit)
	if position >= limit || !isAddressAtom(tokens[position]) {
		return DeliveryPoint{}, start, false
	}

	delivery := DeliveryPoint{
		Kind: DeliveryPointPostal,
		Postal: PostalDelivery{
			Type:   postalType,
			Number: tokens[position].value,
		},
	}
	return delivery, position + 1, true
}

func recognizeStreet(tokens []token, start, limit int) (DeliveryPoint, int, bool) {
	start = skipSoftTokensBefore(tokens, start, limit)
	streetLimit := nextPostalStart(tokens, start+1, limit)
	if streetLimit < 0 {
		streetLimit = limit
	}

	position := start
	delivery := StreetDelivery{}

	first, firstNext, ok := consumeAtom(tokens, position, streetLimit)
	if !ok {
		return DeliveryPoint{}, start, false
	}
	if first.kind == tokenNumberish {
		slashPosition := skipSoftTokensBefore(tokens, firstNext, streetLimit)
		if slashPosition < streetLimit && tokens[slashPosition].kind == tokenSlash {
			streetNumber, streetNumberNext, numberOK := consumeAtom(tokens, slashPosition+1, streetLimit)
			if !numberOK || streetNumber.kind != tokenNumberish {
				return DeliveryPoint{}, start, false
			}
			delivery.Unit = first.value
			delivery.StreetNumber = streetNumber.value
			position = streetNumberNext
		}
	}

	if delivery.StreetNumber == "" {
		if unitType, next, matched := matchKeyword(tokens, position, streetLimit, unitKeywords); matched {
			identifier, identifierNext, identifierOK := consumeAtom(tokens, next, streetLimit)
			if !identifierOK {
				return DeliveryPoint{}, start, false
			}
			delivery.Unit = strings.TrimSpace(unitType + " " + identifier.value)
			position = identifierNext
		}

		if levelType, next, matched := matchKeyword(tokens, position, streetLimit, levelKeywords); matched {
			if levelNeedsIdentifier(levelType) {
				identifier, identifierNext, identifierOK := consumeAtom(tokens, next, streetLimit)
				if !identifierOK {
					return DeliveryPoint{}, start, false
				}
				delivery.Level = strings.TrimSpace(levelType + " " + identifier.value)
				position = identifierNext
			} else {
				delivery.Level = levelType
				position = next
			}
		} else if level, next, matched := matchCompactLevel(tokens, position, streetLimit); matched {
			delivery.Level = level
			position = next
		}

		streetNumber, next, numberOK := consumeAtom(tokens, position, streetLimit)
		if !numberOK || streetNumber.kind != tokenNumberish {
			return DeliveryPoint{}, start, false
		}
		delivery.StreetNumber = streetNumber.value
		position = next
	}

	atoms := make([]token, 0, streetLimit-position)
	for position < streetLimit {
		current := tokens[position]
		position++
		if isSoftToken(current) {
			continue
		}
		if !isAddressAtom(current) {
			return DeliveryPoint{}, start, false
		}
		atoms = append(atoms, current)
	}
	if len(atoms) == 0 {
		return DeliveryPoint{}, start, false
	}

	if normalized, suffix := streetSuffixes[atoms[len(atoms)-1].value]; suffix && len(atoms) > 1 {
		delivery.StreetSuffix = normalized
		atoms = atoms[:len(atoms)-1]
	}
	if normalized, streetType := streetTypes[atoms[len(atoms)-1].value]; streetType && len(atoms) > 1 {
		delivery.StreetType = normalized
		atoms = atoms[:len(atoms)-1]
	}
	if len(atoms) == 0 {
		return DeliveryPoint{}, start, false
	}

	name := make([]string, len(atoms))
	for i, atom := range atoms {
		name[i] = atom.value
	}
	delivery.StreetName = strings.Join(name, " ")

	return DeliveryPoint{Kind: DeliveryPointStreet, Street: delivery}, streetLimit, true
}

func matchCompactLevel(tokens []token, start, limit int) (string, int, bool) {
	position := skipSoftTokensBefore(tokens, start, limit)
	if position >= limit || tokens[position].kind != tokenNumberish {
		return "", start, false
	}

	value := tokens[position].value
	bestPrefix := ""
	bestLevelType := ""
	for keyword, levelType := range levelTypes {
		keyword = normalizeAddressAtom(strings.Join(strings.Fields(keyword), " "))
		if strings.Contains(keyword, " ") || !levelNeedsIdentifier(levelType) {
			continue
		}
		identifier := strings.TrimPrefix(value, keyword)
		if identifier == value || identifier == "" || !isNumberish(identifier) {
			continue
		}
		if len(keyword) > len(bestPrefix) {
			bestPrefix = keyword
			bestLevelType = levelType
		}
	}
	if bestPrefix == "" {
		return "", start, false
	}

	streetNumber, _, ok := consumeAtom(tokens, position+1, limit)
	if !ok || streetNumber.kind != tokenNumberish {
		return "", start, false
	}

	identifier := strings.TrimPrefix(value, bestPrefix)
	return bestLevelType + " " + identifier, position + 1, true
}

func recognizeDeliverySequence(tokens []token, start, limit int) ([]DeliveryPoint, int, bool) {
	position := skipSoftTokensBefore(tokens, start, limit)
	points := make([]DeliveryPoint, 0, 2)

	for position < limit {
		if postal, next, ok := recognizePostal(tokens, position, limit); ok {
			points = append(points, postal)
			position = skipSoftTokensBefore(tokens, next, limit)
			continue
		}
		if street, next, ok := recognizeStreet(tokens, position, limit); ok {
			points = append(points, street)
			position = skipSoftTokensBefore(tokens, next, limit)
			continue
		}
		return nil, start, false
	}

	return points, position, len(points) > 0
}

func nextPostalStart(tokens []token, start, limit int) int {
	for position := skipSoftTokensBefore(tokens, start, limit); position < limit; position++ {
		if _, _, ok := recognizePostal(tokens, position, limit); ok {
			return position
		}
	}
	return -1
}

func consumeAtom(tokens []token, start, limit int) (token, int, bool) {
	position := skipSoftTokensBefore(tokens, start, limit)
	if position >= limit || !isAddressAtom(tokens[position]) {
		return token{}, start, false
	}
	return tokens[position], position + 1, true
}

func skipSoftTokensBefore(tokens []token, position, limit int) int {
	for position < limit && isSoftToken(tokens[position]) {
		position++
	}
	return position
}

func isSoftToken(current token) bool {
	return current.kind == tokenComma || current.kind == tokenNewline
}

func isAddressAtom(current token) bool {
	return current.kind == tokenWord || current.kind == tokenNumberish
}

func levelNeedsIdentifier(levelType string) bool {
	switch levelType {
	case "L", "FL":
		return true
	default:
		return false
	}
}

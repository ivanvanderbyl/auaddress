package auaddress

import "strings"

//go:generate go run ./cmd/locality-gen -output localities_generated.go

type stateMask uint8

const (
	stateNSW stateMask = 1 << iota
	stateVIC
	stateQLD
	stateSA
	stateWA
	stateTAS
	stateACT
	stateNT
)

var stateMasks = map[string]stateMask{
	"NSW": stateNSW,
	"VIC": stateVIC,
	"QLD": stateQLD,
	"SA":  stateSA,
	"WA":  stateWA,
	"TAS": stateTAS,
	"ACT": stateACT,
	"NT":  stateNT,
}

func (mask stateMask) contains(state string) bool {
	stateBit, ok := stateMasks[state]
	return ok && mask&stateBit != 0
}

type localityMatch struct {
	name   string
	states stateMask
	next   int
}

func matchLocality(tokens []token, start int) (localityMatch, bool) {
	position := skipSoftTokens(tokens, start)
	parts := make([]string, 0, maxLocalityTokens)
	best := localityMatch{}
	found := false

	for position < len(tokens) && len(parts) < maxLocalityTokens {
		current := tokens[position]
		if current.kind != tokenWord && current.kind != tokenNumberish {
			break
		}

		parts = append(parts, current.value)
		position++

		name := strings.Join(parts, " ")
		if states, ok := localityStates[name]; ok {
			best = localityMatch{name: name, states: states, next: position}
			found = true
		}

		position = skipSoftTokens(tokens, position)
	}

	return best, found
}

func skipSoftTokens(tokens []token, position int) int {
	for position < len(tokens) {
		switch tokens[position].kind {
		case tokenComma, tokenNewline:
			position++
		default:
			return position
		}
	}
	return position
}

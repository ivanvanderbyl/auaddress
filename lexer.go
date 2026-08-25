package auaddress

import (
	"fmt"
	"strings"
	"text/scanner"
	"unicode"
)

type tokenKind uint8

const (
	tokenWord tokenKind = iota + 1
	tokenNumberish
	tokenSlash
	tokenComma
	tokenNewline
	tokenEOF
)

type token struct {
	kind   tokenKind
	value  string
	raw    string
	offset int
	end    int
	line   int
	column int
}

func lexAddress(raw string) ([]token, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	var scan scanner.Scanner
	scan.Init(strings.NewReader(normalized))
	scan.Mode = scanner.ScanIdents
	scan.Whitespace = scanner.GoWhitespace &^ (1 << '\n')
	scan.IsIdentRune = isAddressIdentRune

	var scanErr error
	scan.Error = func(s *scanner.Scanner, message string) {
		if scanErr == nil {
			scanErr = fmt.Errorf("address token at %d:%d: %s", s.Position.Line, s.Position.Column, message)
		}
	}

	tokens := make([]token, 0, len(normalized)/4+1)
	for {
		kind := scan.Scan()
		position := scan.Position
		rawToken := scan.TokenText()

		switch kind {
		case scanner.EOF:
			tokens = append(tokens, token{
				kind:   tokenEOF,
				offset: len(normalized),
				end:    len(normalized),
				line:   position.Line,
				column: position.Column,
			})
			return tokens, scanErr
		case scanner.Ident:
			value := normalizeAddressAtom(rawToken)
			if value == "" {
				continue
			}
			tokenKind := tokenWord
			if isNumberish(value) {
				tokenKind = tokenNumberish
			}
			tokens = append(tokens, token{
				kind:   tokenKind,
				value:  value,
				raw:    rawToken,
				offset: position.Offset,
				end:    position.Offset + len(rawToken),
				line:   position.Line,
				column: position.Column,
			})
		case '/':
			tokens = append(tokens, positionedToken(tokenSlash, "/", rawToken, position))
		case ',', ';':
			tokens = append(tokens, positionedToken(tokenComma, ",", rawToken, position))
		case '\n':
			tokens = append(tokens, positionedToken(tokenNewline, "\n", rawToken, position))
		case '.', ':':
			// Punctuation outside an atom is a soft separator.
		default:
			value := strings.TrimSpace(rawToken)
			if value != "" {
				tokens = append(tokens, positionedToken(tokenWord, strings.ToUpper(value), rawToken, position))
			}
		}
	}
}

func positionedToken(kind tokenKind, value, raw string, position scanner.Position) token {
	return token{
		kind:   kind,
		value:  value,
		raw:    raw,
		offset: position.Offset,
		end:    position.Offset + len(raw),
		line:   position.Line,
		column: position.Column,
	}
}

func isAddressIdentRune(ch rune, index int) bool {
	if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
		return true
	}
	if index == 0 {
		return false
	}
	return ch == '-' || ch == '\'' || ch == '’' || ch == '.'
}

func normalizeAddressAtom(value string) string {
	value = strings.ReplaceAll(value, ".", "")
	return strings.ToUpper(value)
}

func isNumberish(value string) bool {
	seenDigit := false
	for _, ch := range value {
		switch {
		case unicode.IsDigit(ch):
			seenDigit = true
		case unicode.IsLetter(ch), ch == '-':
		default:
			return false
		}
	}
	return seenDigit
}

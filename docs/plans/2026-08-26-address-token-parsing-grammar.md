# Address Token Parsing Grammar Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the line-based regular-expression parser with a greedy left-to-right token grammar that accepts locality-terminated partial addresses and preserves ordered street and postal delivery points.

**Architecture:** Use `text/scanner` to produce normalized tokens with source positions and soft comma/newline boundaries. Parse those tokens with a deterministic cursor, longest-match vocabulary lookups, and an embedded locality-to-state-mask index, then project the canonical ordered result into the existing flat API.

**Tech Stack:** Go standard library, Go 1.25 module compatibility, generated ABS locality data, table-driven tests, and the existing G-NAF regression suite.

---

### Task 1: Add the canonical delivery-point API

**Files:**
- Modify: `address.go`
- Test: `address_test.go`

**Step 1: Write the failing compatibility and formatting tests**

Add a test that constructs a `ParsedAddress` containing ordered street and
postal delivery points, then assert that `FormatDeliveryLines()` returns both,
`FormatDeliveryLine()` returns the first, and `Format()` includes both before
the locality. Also assert that existing flat fields continue to format when
`DeliveryPoints` is empty.

```go
addr := &ParsedAddress{
    DeliveryPoints: []DeliveryPoint{
        {Kind: DeliveryPointStreet, Street: StreetDelivery{
            StreetNumber: "123", StreetName: "MAIN", StreetType: "ST",
        }},
        {Kind: DeliveryPointPostal, Postal: PostalDelivery{
            Type: "PO BOX", Number: "42",
        }},
    },
    Locality: "RICHMOND",
}
```

**Step 2: Run the focused test and verify it fails**

Run: `go test ./... -run 'TestFormatMixedDeliveryPoints|TestFormat'`

Expected: FAIL because the delivery-point types and method do not exist.

**Step 3: Add the public types and compatibility fallback**

Add `DeliveryPointKind`, `StreetDelivery`, `PostalDelivery`, `DeliveryPoint`,
and `ParsedAddress.DeliveryPoints`. Implement `FormatDeliveryLines()` using the
ordered slice. Keep a private legacy formatter for an address with an empty
ordered slice. Make `FormatDeliveryLine()` return the first canonical line, or
the legacy line when needed.

**Step 4: Run the focused tests and verify they pass**

Run: `go test ./... -run 'TestFormatMixedDeliveryPoints|TestFormat'`

Expected: PASS.

**Step 5: Commit**

```bash
git add address.go address_test.go
git commit -m "Add ordered delivery point model"
```

### Task 2: Introduce address tokenization

**Files:**
- Create: `lexer.go`
- Create: `lexer_test.go`

**Step 1: Write table-driven lexer tests**

Cover words, number-like identifiers, slash notation, ranges, punctuation,
Windows line endings, and split components. Expected tokens must include
normalized values and explicit comma/newline kinds for an input such as
`3A/10-12 Main\nStreet, Richmond`.

**Step 2: Run lexer tests and verify they fail**

Run: `go test ./... -run TestLexAddress`

Expected: FAIL because the lexer does not exist.

**Step 3: Implement the lexer**

Configure `text/scanner.Scanner` so spaces and tabs are skipped but newlines are
returned. Use `IsIdentRune` to keep Unicode letters, digits, apostrophes,
periods, and internal hyphens in address atoms. Classify atoms by rune content,
uppercase them, normalize CRLF, and retain byte offset, line, and column.

**Step 4: Run lexer tests and the package suite**

Run: `go test ./... -run TestLexAddress && go test ./...`

Expected: PASS.

**Step 5: Commit**

```bash
git add lexer.go lexer_test.go
git commit -m "Add address token lexer"
```

### Task 3: Generate the locality and state-mask index

**Files:**
- Create: `locality.go`
- Create: `locality_test.go`
- Create: `cmd/locality-gen/main.go`
- Create: `localities_generated.go`

**Step 1: Write failing locality lookup tests**

Test a unique locality, a multi-word locality, and a cross-state duplicate. The
matcher must return the longest name, correct state mask, and consumed token
count even when a newline separates locality words.

```go
tokens, _ := lexAddress("ALICE\nSPRINGS NT")
match, ok := matchLocality(tokens, 0)
if !ok || match.name != "ALICE SPRINGS" || !match.states.contains("NT") {
    t.Fatalf("unexpected locality match: %#v, %v", match, ok)
}
```

**Step 2: Run the test and verify it fails**

Run: `go test ./... -run TestMatchLocality`

Expected: FAIL because locality matching does not exist.

**Step 3: Implement locality normalization and state masks**

Define one bit per supported state/territory and helpers to create and query a
mask. Implement longest-match lookup over word/numberish tokens while skipping
soft boundaries. Store the generated maximum locality token count.

**Step 4: Implement and run the generator**

The generator must page through the official ABS SAL ArcGIS endpoint, strip
documented disambiguation suffixes, merge duplicate normalized names by OR-ing
state bits, sort names, and emit formatted deterministic Go source with the
source URL and edition in its header.

Run: `go run ./cmd/locality-gen -output localities_generated.go`

Expected: `localities_generated.go` contains roughly 14,000 unique names and
does not change when the command is run twice against the same source edition.

**Step 5: Run locality tests**

Run: `go test ./... -run TestMatchLocality`

Expected: PASS.

**Step 6: Commit**

```bash
git add locality.go locality_test.go cmd/locality-gen/main.go localities_generated.go
git commit -m "Embed Australian locality index"
```

### Task 4: Implement delivery-point recognizers

**Files:**
- Create: `grammar.go`
- Create: `grammar_test.go`
- Modify: `data.go`

**Step 1: Write failing recognizer tests**

Use table-driven tests for `3A/45 High Street`,
`Unit 5 Level 2 100 George Street`, `10-12 King George Road North`,
`PO Box 42`, punctuated `P.O. Box 42`, `Locked Bag 5000`, and street/postal
delivery points in both orders. Assert consumed tokens and structured results.

**Step 2: Run recognizer tests and verify they fail**

Run: `go test ./... -run 'TestRecognizeStreet|TestRecognizePostal|TestRecognizeDeliverySequence'`

Expected: FAIL because the grammar recognizers do not exist.

**Step 3: Implement the token cursor and keyword matching**

Add cursor operations for peek, consume, mark, restore within one recognizer,
and soft-boundary skipping. Convert postal types and multi-token aliases into
ordered keyword entries. Reuse existing unit, level, street type, and suffix
normalization maps.

**Step 4: Implement postal and street recognizers**

Postal recognition consumes the longest postal type and one delivery
identifier. Street recognition consumes optional unit/level data, a required
street number, one or more street-name tokens, an optional normalized street
type, and an optional suffix. It stops only at a valid next delivery point or a
locality that permits a valid address tail.

**Step 5: Run recognizer tests**

Run: `go test ./... -run 'TestRecognizeStreet|TestRecognizePostal|TestRecognizeDeliverySequence'`

Expected: PASS.

**Step 6: Commit**

```bash
git add grammar.go grammar_test.go data.go
git commit -m "Add address token grammar recognizers"
```

### Task 5: Replace line-position parsing with the token grammar

**Files:**
- Modify: `address.go`
- Modify: `address_test.go`

**Step 1: Add failing end-to-end parsing tests**

Add table-driven cases for:

```text
123 Main Street, Richmond
123 Main\nStreet\nRichmond VIC 3121
Company\nPO Box 42\nRichmond
123 Main Street, PO Box 42, Richmond VIC 3121
PO Box 42\n123 Main Street\nRichmond VIC 3121
```

Assert locality recognition, optional state/postcode, ordered delivery points,
legacy field projection, prefix `NameLines`, and formatting. Add rejection
cases for `123 Main Street`, an unknown locality, incompatible state, malformed
explicit postcode, and trailing junk.

**Step 2: Run focused parsing tests and verify they fail**

Run: `go test ./... -run 'TestParsePartialAddress|TestParseSplitAddress|TestParseMixedDeliveryPoints'`

Expected: FAIL under the line-position parser.

**Step 3: Implement the address grammar driver**

Replace `parseLastLine`, `parseDeliveryLine`, and regex-based component parsing
with a token driver that finds the first delivery-point start, preserves the
preceding source as recipient/building lines, greedily consumes delivery
points, consumes the longest valid locality, validates optional state/postcode,
requires EOF, and projects the first delivery points into legacy fields.

Keep lenient mode collecting the first grammar or validation error and strict
mode returning it. Do not add errors when state or postcode are absent.

**Step 4: Run focused and complete tests**

Run: `go test ./... -run 'TestParsePartialAddress|TestParseSplitAddress|TestParseMixedDeliveryPoints|TestStrictMode|TestFormat'`

Expected: PASS.

Run: `go test ./...`

Expected: PASS, including G-NAF regression tests when fixture data is present.

**Step 5: Commit**

```bash
git add address.go address_test.go grammar.go
git commit -m "Parse addresses with token grammar"
```

### Task 6: Update validation and compatibility behavior

**Files:**
- Modify: `address.go`
- Modify: `address_test.go`

**Step 1: Write failing validity tests**

Assert that street or postal delivery plus a known locality is valid without
state/postcode; delivery without locality and locality without delivery are
invalid; mixed delivery satisfies `HasDeliveryPoint`; and manually constructed
legacy values retain helper behavior.

**Step 2: Run the tests and verify they fail**

Run: `go test ./... -run 'TestIsValid|TestHasDeliveryPoint'`

Expected: at least the new partial-address cases FAIL.

**Step 3: Update helper semantics**

Make `IsValid()` require no errors, a recognized locality, and a delivery point.
Make `HasDeliveryPoint()` prefer `DeliveryPoints` and fall back to legacy fields
for caller-constructed values.

**Step 4: Run helper and full tests**

Run: `go test ./... -run 'TestIsValid|TestHasDeliveryPoint' && go test ./...`

Expected: PASS.

**Step 5: Commit**

```bash
git add address.go address_test.go
git commit -m "Update partial address validation"
```

### Task 7: Document the address token parsing grammar

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-08-26-address-token-parsing-grammar-design.md`

**Step 1: Update public documentation**

Document the address token parsing grammar by name, partial validity, soft line
boundaries, mixed delivery points, `DeliveryPoints`, optional state/postcode,
locality data source, and compatibility fields. Update examples and the parsed
structure listing.

**Step 2: Check documentation claims against tests and code**

Run: `rg -n 'address token parsing grammar|DeliveryPoints|partial|state|postcode' README.md docs/plans`

Expected: each public behavior is described and matches an automated test.

**Step 3: Run formatting and tests**

Run: `gofmt -w *.go cmd/locality-gen/*.go && go test ./...`

Expected: PASS.

**Step 4: Commit**

```bash
git add README.md docs/plans/2026-08-26-address-token-parsing-grammar-design.md
git commit -m "Document token grammar parser"
```

### Task 8: Verify correctness, determinism, and performance

**Files:**
- Modify: `gnaf_test.go`
- Modify: `locality_test.go`

**Step 1: Add generator and benchmark coverage**

Test locality entry count and representative state masks so accidental partial
generation fails visibly. Add benchmark inputs for single-line partial, split,
and mixed delivery addresses.

**Step 2: Run all static and dynamic verification**

Run:

```bash
gofmt -w *.go cmd/locality-gen/*.go
git diff --check
go vet ./...
go test -count=1 ./...
go test -race ./...
go test -run '^$' -bench 'BenchmarkGNAFParsing|BenchmarkTokenGrammar' -benchmem ./...
```

Expected: formatting and vet are clean; tests and race tests pass; benchmark
results are recorded without imposing an arbitrary threshold.

**Step 3: Review the final diff and public API**

Run:

```bash
git status --short
git diff --stat main...HEAD
git diff main...HEAD -- address.go lexer.go grammar.go locality.go README.md
```

Expected: only parser, locality data/generator, tests, and documentation are in
scope; no unrelated files changed.

**Step 4: Commit final verification additions**

```bash
git add gnaf_test.go locality_test.go
git commit -m "Add token grammar regression benchmarks"
```


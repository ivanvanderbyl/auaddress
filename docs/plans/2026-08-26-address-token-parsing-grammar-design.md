# Address token parsing grammar design

## Goal

Replace the line-position and regular-expression parser with a greedy,
left-to-right address token parsing grammar. The parser must:

- accept an address once it contains at least one delivery point and a known
  Australian locality;
- treat state and postcode as optional trailing components;
- parse components across arbitrary spaces, commas, and line breaks;
- preserve street and postal delivery points when both appear; and
- parse multiple independently locality-terminated addresses from one input;
- compare parsed addresses as exact, partial, or conflicting normalized keys;
- retain the existing public `ParsedAddress` fields and methods for current
  callers.

Examples of valid partial addresses include `123 Main Street, Richmond` and
`PO Box 42 Richmond`. `123 Main Street` is incomplete because it has no known
locality.

## Architecture

The parser has five stages:

1. Normalize line endings and whitespace without losing comma and newline
   boundary positions.
2. Lex the input into address tokens with Go's `text/scanner` package.
3. Parse the token stream with a deterministic cursor and ordered component
   recognizers.
4. Emit each independently locality-terminated address and continue scanning
   from the next delivery-point start.
5. Project each parsed result into both the canonical ordered model and the
   legacy flat fields.

The implementation does not use global backtracking. A recognizer may use
bounded lookahead to match a multi-token keyword or the longest known locality.
Each successful recognizer advances the cursor, so parsing remains greedy and
left to right.

Regular expressions are removed from the parsing path. Keyword tables and
small rune classifiers recognize atomic token shapes.

## Token model

The lexer emits these token kinds:

- `word`: names, keywords, and alphanumeric identifiers;
- `numberish`: street and delivery identifiers such as `3A` and `10-12`;
- `slash`: unit/street separators;
- `comma`: a soft component boundary;
- `newline`: a soft component boundary; and
- `EOF`.

Each token retains its original source span and normalized uppercase value.
Commas and newlines are hints, not hard grammar boundaries. A component may
continue across either boundary where its recognizer permits it.

## Grammar

```text
AddressSequence :=
    RecipientOrBuilding*
    Address+
    EOF

Address :=
    DeliveryPoint+
    Locality
    State?
    Postcode?

DeliveryPoint := StreetDelivery | PostalDelivery

StreetDelivery :=
    Unit? Level? StreetNumber StreetName StreetType? StreetSuffix?

PostalDelivery :=
    PostalType DeliveryIdentifier
```

The parser preserves prefix text before the first valid delivery-point start
as recipient or building data and copies it to every result. It then consumes
one or more delivery points in encounter order. A locality match followed by an
optional state and postcode terminates one address when the next token is EOF
or begins another delivery point. The parser then emits that address and
continues left to right.

Multiple delivery points followed by one locality produce one `ParsedAddress`.
Delivery points that each have their own locality terminator produce separate
results. Commas and newlines influence ambiguous boundaries but do not determine
the number of results.

State validation uses the matched locality's state bitmask. A locality without
a state is valid. If a state is present, the corresponding bit must be set. A
postcode remains syntactically validated as four digits; postcode-to-locality
validation is outside this change.

The parser consumes address text rather than searching arbitrary email text.
Every non-boundary token must belong to the shared prefix or one parsed
address; unexplained text is invalid.

## Locality index

The package embeds normalized Australian locality names with an eight-bit
state mask. Duplicate locality names across states collapse into one entry with
multiple bits. Duplicate official areas with the same name in one state share
one bit.

The generated data records its source and edition. A generator reads primary
and alias locality PSV tables from the official Geoscape G-NAF archive and
merges them into the Go table. It can read a local archive or use HTTP range
requests so refreshing the index does not download unrelated address tables.
Callers do not need the source dataset or a network connection at build or run
time.

The generated index currently contains 21,852 primary and alias names. Runtime
memory and binary size are measured during final verification rather than
constrained by an arbitrary threshold.

## Public model

Add an ordered delivery model:

```go
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
```

`ParsedAddress.DeliveryPoints` is the canonical delivery representation. The
existing fields remain a compatibility projection:

- street fields mirror the first street delivery;
- postal fields mirror the first postal delivery; and
- `IsPoBox` is true when any postal delivery exists.

Add `FormatDeliveryLines() []string`. `Format()` emits every delivery point on
its own line in encounter order. `FormatDeliveryLine()` returns the first
delivery point and remains unchanged for existing single-delivery addresses.

`HasDeliveryPoint()` checks the canonical ordered collection. `IsValid()`
requires no parsing errors, at least one delivery point, and a recognized
locality. State and postcode are optional.

Add multi-address entry points:

```go
func ParseAll(raw string) ([]*ParsedAddress, error)
func (p *Parser) ParseAll(raw string) ([]*ParsedAddress, error)
```

`ParseAll` returns one result per independently locality-terminated address.
Existing `Parse` delegates to `ParseAll` and returns the first result, preserving
its signature and single-address behavior.

## Normalised comparison

Each parsed address can produce a deterministic comparison key from normalized
delivery, locality, state, and postcode components. The key retains component
names and missing values so it is suitable for logging, indexing, and exact
comparison without relying on display formatting.

Comparison returns:

```go
type MatchKind uint8

const (
    NoMatch MatchKind = iota
    PartialMatch
    ExactMatch
)

type AddressMatch struct {
    Kind             MatchKind
    MatchedThrough   MatchComponent
    MissingFromLeft  []MatchComponent
    MissingFromRight []MatchComponent
    LeftKey          string
    RightKey         string
}
```

An exact match has identical normalized populated components. A partial match
has no conflicting overlapping components but omits specificity on one or both
sides, including unit, level, state, or postcode. A mismatch has at least one
conflicting identity component or incompatible delivery kind.

`MatchedThrough` reports the furthest ordered component present and equal on
both sides. Missing-component lists explain the specificity difference. This
version deliberately excludes edit distance, phonetic matching, and typo
correction.

## Error handling

Lenient mode returns the recognized partial structure and records grammar or
validation failures in `ParsedAddress.Errors`. Strict mode returns the first
such error immediately.

Existing sentinel errors remain available. Their updated meanings are:

- `ErrNoDeliveryLine`: no street or postal delivery point was recognized;
- `ErrInvalidAddress`: the token sequence does not satisfy the grammar;
- `ErrNoState`: an explicit state is unknown or incompatible with the locality;
- `ErrNoPostcode`: an explicit postcode token is malformed; and
- `ErrEmptyAddress`: no address tokens were supplied.

Omitted state and postcode do not create errors. A missing or unknown locality
does create an invalid-address error because locality is the minimum required
terminator.

`ParseAll` must consume the entire input. In strict mode it returns the
successfully parsed prefix plus the first segmentation or validation error. In
lenient mode it records the error on the affected result where possible. It
never silently treats unexplained trailing content as another address.

## Repository automation

A root `Taskfile.yml` provides a task that resolves the latest GDA2020 G-NAF
release from data.gov.au and regenerates the locality index. A guarded release
task requires an explicit semantic version, a clean worktree, successful tests,
and deterministic generated data before creating a version tag. The build does
not execute or push a release tag automatically.

## Testing

Table-driven tests cover:

- all existing single street and postal examples;
- locality-only completion with omitted state and postcode;
- rejection of a delivery point without a locality;
- components split at every token boundary across spaces, commas, and newlines;
- street then postal, and postal then street, on one or several lines;
- two independently terminated addresses on one line or arbitrary lines;
- one shared prefix copied to every parsed address;
- multiple delivery points sharing one locality remain one result;
- complete input consumption and rejection of unexplained text;
- longest multi-word locality matches;
- duplicate locality names with absent, matching, and conflicting states;
- street/locality vocabulary collisions;
- unit slash notation, levels, street ranges, suffixes, and multi-word types;
- strict and lenient error behavior;
- canonical and compatibility field projection; and
- ordered formatting and round trips;
- exact normalized-key comparison;
- trailing state/postcode partial matches;
- missing unit/level specificity matches; and
- conflicting delivery and locality components.

The existing G-NAF accuracy tests remain regression coverage. Benchmarks compare
the new scanner with the current baseline and report allocations and throughput;
correctness takes precedence over preserving the original microbenchmark.

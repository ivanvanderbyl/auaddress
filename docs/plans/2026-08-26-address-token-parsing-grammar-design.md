# Address Token Parsing Grammar Design

## Goal

Replace the line-position and regular-expression parser with a greedy,
left-to-right address token parsing grammar. The parser must:

- accept an address once it contains at least one delivery point and a known
  Australian locality;
- treat state and postcode as optional trailing components;
- parse components across arbitrary spaces, commas, and line breaks;
- preserve street and postal delivery points when both appear; and
- retain the existing public `ParsedAddress` fields and methods for current
  callers.

Examples of valid partial addresses include `123 Main Street, Richmond` and
`PO Box 42 Richmond`. `123 Main Street` is incomplete because it has no known
locality.

## Architecture

The parser has four stages:

1. Normalize line endings and whitespace without losing comma and newline
   boundary positions.
2. Lex the input into address tokens with Go's `text/scanner` package.
3. Parse the token stream with a deterministic cursor and ordered component
   recognizers.
4. Project the parsed components into both the canonical ordered model and the
   legacy flat fields.

The implementation does not use global backtracking. A recognizer may use
bounded lookahead to match a multi-token keyword or the longest known locality.
Each successful recognizer advances the cursor, so parsing remains greedy and
left to right.

Regular expressions are removed from the parsing path. Keyword tables and
small rune classifiers recognize atomic token shapes.

## Token Model

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
Address :=
    RecipientOrBuilding*
    DeliveryPoint+
    Locality
    State?
    Postcode?
    EOF

DeliveryPoint := StreetDelivery | PostalDelivery

StreetDelivery :=
    Unit? Level? StreetNumber StreetName StreetType? StreetSuffix?

PostalDelivery :=
    PostalType DeliveryIdentifier
```

The parser preserves prefix text before the first valid delivery-point start
as recipient or building data. It then consumes one or more delivery points in
encounter order. The first longest locality match that permits a valid trailing
state, postcode, or end of input terminates delivery parsing.

State validation uses the matched locality's state bitmask. A locality without
a state is valid. If a state is present, the corresponding bit must be set. A
postcode remains syntactically validated as four digits; postcode-to-locality
validation is outside this change.

Trailing non-boundary tokens after the optional postcode are invalid.

## Locality Index

The package embeds normalized Australian locality names with an eight-bit
state mask. Duplicate locality names across states collapse into one entry with
multiple bits. Duplicate official areas with the same name in one state share
one bit.

The generated data records its source and edition. A generator creates the Go
table from the official source; callers do not need the source dataset or a
network connection at build or run time.

The target runtime footprint is less than 1 MB for the locality membership and
state index.

## Public Model

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

## Error Handling

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

## Testing

Table-driven tests cover:

- all existing single street and postal examples;
- locality-only completion with omitted state and postcode;
- rejection of a delivery point without a locality;
- components split at every token boundary across spaces, commas, and newlines;
- street then postal, and postal then street, on one or several lines;
- longest multi-word locality matches;
- duplicate locality names with absent, matching, and conflicting states;
- street/locality vocabulary collisions;
- unit slash notation, levels, street ranges, suffixes, and multi-word types;
- strict and lenient error behavior;
- canonical and compatibility field projection; and
- ordered formatting and round trips.

The existing G-NAF accuracy tests remain regression coverage. Benchmarks compare
the new scanner with the current baseline and report allocations and throughput;
correctness takes precedence over preserving the original microbenchmark.


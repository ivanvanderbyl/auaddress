# auaddress

Parse one or more Australian address fragments from an address-tagged string.
The greedy, left-to-right address token parsing grammar accepts an address once
it finds a street or postal delivery point followed by a known locality. State
and postcode are optional.

The parser treats commas and line breaks as soft boundaries. It can recover an
address split across lines, keep a street and PO box that share one locality,
or return two addresses from one model tag.

## Install

```bash
go get github.com/ivanvanderbyl/auaddress
```

## Parse one address

`Parse` returns the first address in the input. A partial address is valid when
it has a delivery point and a locality:

```go
addr, err := auaddress.Parse("123 Main Street, Richmond")
if err != nil {
    return err
}

fmt.Println(addr.StreetNumber) // 123
fmt.Println(addr.StreetName)   // MAIN
fmt.Println(addr.StreetType)   // ST
fmt.Println(addr.Locality)     // RICHMOND
fmt.Println(addr.State)        // empty
fmt.Println(addr.Postcode)     // empty
```

`123 Main Street` is not valid because it has no recognised locality. This
allows callers to omit a state or country when the recipient's context already
supplies it without accepting a bare delivery line.

## Parse every address in a tag

Use `ParseAll` when an address recognition model may return more than one
address in one address (`ADR`) tag:

```go
addresses, err := auaddress.ParseAll(`School Infrastructure NSW
Level 8, 259 George Street, Sydney, NSW 2000
GPO Box 33, Sydney, NSW 2001`)
if err != nil {
    return err
}

fmt.Println(len(addresses))          // 2
fmt.Println(addresses[0].Postcode)   // 2000
fmt.Println(addresses[1].PoBoxType)  // GPO BOX
fmt.Println(addresses[1].Postcode)   // 2001
```

Physical lines do not determine the result count. Each independently
locality-terminated delivery sequence becomes one result. The parser copies
shared recipient or organisation lines to every result.

When multiple delivery points share one locality, they remain one address:

```go
addr, _ := auaddress.Parse("123 Main Street, PO Box 42, Richmond VIC 3121")

fmt.Println(len(addr.DeliveryPoints)) // 2
fmt.Println(addr.FormatDeliveryLines())
// 123 MAIN ST
// PO BOX 42
```

## Compare parsed addresses

`ComparisonKey` exposes the normalised components as a deterministic key.
`CompareAddresses` classifies two parsed addresses as `ExactMatch`,
`PartialMatch`, or `NoMatch`.

```go
left, _ := auaddress.Parse("123 Main Street, Richmond")
right, _ := auaddress.Parse("123 Main St, Richmond VIC 3121")

match := auaddress.CompareAddresses(left, right)
fmt.Println(match.Kind == auaddress.PartialMatch) // true
fmt.Println(match.MatchedThrough == auaddress.MatchLocality) // true
fmt.Println(match.MissingFromLeft[0] == auaddress.MatchState) // true
fmt.Println(match.MissingFromLeft[1] == auaddress.MatchPostcode) // true
```

A missing unit, level, state, or postcode makes the match partial. Conflicting
populated components, such as different street numbers or localities, produce
`NoMatch`. Comparison is component-based: it does not use edit distance,
phonetic matching, or typo correction.

## Work with delivery points

`DeliveryPoints` is the canonical ordered representation. The flat street and
PO box fields remain populated for existing callers.

```go
type DeliveryPoint struct {
    Kind   DeliveryPointKind
    Street StreetDelivery
    Postal PostalDelivery
}

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
```

The compatibility projection uses the first street delivery point and the
first postal delivery point. Use `DeliveryPoints` when encounter order or more
than one delivery point matters.

## Choose error handling

The default parser returns recognised structure and records the first grammar
or validation failure in `ParsedAddress.Errors`:

```go
addr, err := auaddress.Parse(input)
if err != nil {
    return err // empty input is always returned directly
}
if len(addr.Errors) > 0 {
    // Inspect a lenient parsing failure.
}
```

Strict mode returns the first failure directly. With `ParseAll`, it also
returns any complete addresses parsed before that failure:

```go
parser := auaddress.NewParser(auaddress.WithStrict(true))
addresses, err := parser.ParseAll(input)
```

An address is valid when it has no recorded errors, at least one delivery
point, and a recognised locality:

```go
if addr.IsValid() {
    // The string satisfies the grammar.
}
```

## Supported components

- States: NSW, VIC, QLD, SA, WA, TAS, ACT, and NT.
- Street types: G-NAF forms including STREET/ST, ROAD/RD, AVENUE/AV,
  DRIVE/DR, PLACE/PL, COURT/CT, and their normalised abbreviations.
- Unit types: UNIT, FLAT, APARTMENT/APT, VILLA, LOT, SHOP, SUITE, ROOM,
  OFFICE, FACTORY, WAREHOUSE, and related G-NAF forms.
- Level types: LEVEL/L, FLOOR/FL, GROUND/G, BASEMENT/B, MEZZANINE/M,
  LOWER GROUND/LG, UPPER GROUND/UG, and related forms.
- Postal delivery types: PO BOX, GPO BOX, LOCKED BAG, PRIVATE BAG, REPLY
  PAID, RMB, CMB, RSD, MS, CMA, CPA, and CARE PO.

## Locality data

The package embeds 21,852 normalised primary and alias locality names from the
Geoscape Geocoded National Address File (G-NAF). Each name stores an eight-bit
state mask, so duplicate locality names across states occupy one index entry.
Parsing does not need a network connection.

Refresh the index from the latest GDA2020 release published in the official
data.gov.au package metadata:

```bash
task gnaf:update
```

The generator reads only the locality tables from the remote ZIP by using HTTP
range requests. It records the resolved archive URL in
`localities_generated.go`.

## Scope and limitations

Pass an ADR-tagged span to this package. It is not a general email-signature
extractor. Prefix text may become shared recipient or organisation lines, but
all content after parsing begins must belong to an address sequence.

The parser validates grammar, known locality names, locality/state
compatibility, and four-digit postcode shape. It does not prove that a street
address exists, validate that a postcode belongs to a locality, query
Australia Post's Postal Address File, handle international addresses, or
provide Address Matching Approval System certification.

## Development and releases

```bash
task test
task gnaf:update
task release VERSION=v1.2.3
task release:push VERSION=v1.2.3
```

`task test` checks Go formatting, runs `go vet`, and runs all tests. `release`
requires a clean worktree and creates an annotated local semantic-version tag.
It never pushes. `release:push` is the separate explicit push step.

## References

- [Australia Post addressing guidelines](https://auspost.com.au/sending/guidelines/addressing-guidelines)
- [Australia Post addressing standards](https://auspost.com.au/content/dam/auspost_corp/media/documents/australia-post-addressing-standards-1999.pdf)
- [AMAS developer guide](https://auspost.com.au/content/dam/auspost_corp/media/documents/amas-developer-guide.pdf)
- [Geocoded National Address File](https://data.gov.au/data/dataset/geocoded-national-address-file-g-naf)

## Licence

MIT

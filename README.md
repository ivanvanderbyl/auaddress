# auaddress

A fast Go package for parsing Australian addresses according to Australia Post addressing standards, with reference data sourced from G-NAF (Geocoded National Address File).

## Features

- **Fast**: ~470,000 addresses/second (2.1μs per address)
- **Accurate**: 100% locality/state/postcode accuracy, 99%+ street parsing accuracy
- **Complete**: Supports all G-NAF street types, unit types, and level types
- **Standards-compliant**: Normalises to AusPost presentation format

## Scope

This library handles **single domestic destination address blocks** with lines separated by `\n`. It normalises reasonably well-formed AU addresses to AusPost's standard format.

### What this library does

- Parses address components (locality, state, postcode, street details, unit/level, PO Box)
- Normalises addresses to AusPost presentation standards (uppercase, standard abbreviations)
- Validates syntactic correctness (valid state codes, 4-digit postcodes)

### What this library does NOT do

- Validate that an address actually exists or is deliverable
- Match against Australia Post's PAF (Postal Address File)
- Handle international addresses
- Guarantee AMAS compliance without external validation

For address existence validation, you'll need an external service like Australia Post's Address Validation API or an AMAS-certified solution.

## Installation

```bash
go get github.com/ivanvanderbyl/auaddress
```

## Usage

### Basic Parsing

```go
package main

import (
    "fmt"
    "github.com/ivanvanderbyl/auaddress"
)

func main() {
    addr, err := auaddress.Parse(`John Smith
123 Main Street
SYDNEY NSW 2000`)
    
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Street: %s %s %s\n", addr.StreetNumber, addr.StreetName, addr.StreetType)
    // Output: Street: 123 MAIN ST
    
    fmt.Printf("Locality: %s %s %s\n", addr.Locality, addr.State, addr.Postcode)
    // Output: Locality: SYDNEY NSW 2000
}
```

### Unit/Level Addresses

```go
addr, _ := auaddress.Parse(`Jane Doe
Unit 5, 100 George Street
BRISBANE QLD 4000`)

fmt.Println(addr.Unit)         // "UNIT 5"
fmt.Println(addr.StreetNumber) // "100"
fmt.Println(addr.StreetName)   // "GEORGE"
fmt.Println(addr.StreetType)   // "ST"
```

### Slash Notation

```go
addr, _ := auaddress.Parse(`Tenant
3A/45 High Street
MELBOURNE VIC 3000`)

fmt.Println(addr.Unit)         // "3A"
fmt.Println(addr.StreetNumber) // "45"
```

### PO Box Addresses

```go
addr, _ := auaddress.Parse(`Company Pty Ltd
PO Box 1234
SYDNEY NSW 2000`)

fmt.Println(addr.IsPoBox)     // true
fmt.Println(addr.PoBoxType)   // "PO BOX"
fmt.Println(addr.PoBoxNumber) // "1234"
```

### Formatting

```go
addr, _ := auaddress.Parse(`john smith
123 main street
sydney nsw 2000`)

fmt.Println(addr.Format())
// Output:
// JOHN SMITH
// 123 MAIN ST
// SYDNEY NSW 2000
```

### Strict Mode

By default, the parser is lenient and collects errors in the `Errors` slice while still returning a partial result. Use strict mode to fail immediately on validation errors:

```go
parser := auaddress.NewParser(auaddress.WithStrict(true))
addr, err := parser.Parse("Invalid Address")
if err != nil {
    // Handle error
}
```

### Validation

```go
addr, _ := auaddress.Parse(input)

if addr.IsValid() {
    // No parsing errors, has valid state and postcode
}

if addr.HasDeliveryPoint() {
    // Has either street address or PO Box
}
```

## Performance

Benchmarked on Apple M4 Max:

```
BenchmarkGNAFParsing-16    575476    2138 ns/op    783 B/op    14 allocs/op
```

- **~470,000 addresses/second**
- **2.1 microseconds per address**
- **783 bytes allocated per parse**

Tested against 5,000 real addresses from G-NAF November 2025:

| Metric | Accuracy |
|--------|----------|
| Locality/State/Postcode | 100% |
| Street Number | 99.8% |
| Street Name | 99.2% |
| Street Type | 99.4% |
| Unit Parsing | 99.3% |

## Parsed Address Structure

```go
type ParsedAddress struct {
    RawLines []string    // Original input lines

    // PO Box / Special delivery
    IsPoBox     bool
    PoBoxType   string   // "PO BOX", "GPO BOX", "LOCKED BAG", etc.
    PoBoxNumber string

    // Street address components
    Unit         string  // "UNIT 5", "3A", etc.
    Level        string  // "L 10", "FL 3", etc.
    StreetNumber string  // "123", "10-12", "45A"
    StreetName   string  // "MAIN", "KING GEORGE"
    StreetType   string  // "ST", "RD", "AV", etc.
    StreetSuffix string  // "N", "S", "E", "W", etc.

    BuildingName string

    // Locality line
    Locality string  // "SYDNEY", "ALICE SPRINGS"
    State    string  // "NSW", "VIC", "QLD", "SA", "WA", "TAS", "ACT", "NT"
    Postcode string  // 4 digits

    // Name/addressee lines
    NameLines []string

    // Parsing errors (in lenient mode)
    Errors []error
}
```

## Supported Formats

### States
NSW, VIC, QLD, SA, WA, TAS, ACT, NT

### Street Types
Complete list from G-NAF including: STREET/ST, ROAD/RD, AVENUE/AV, DRIVE/DR, PLACE/PL, COURT/CT, CIRCUIT/CCT, CRESCENT/CR, TERRACE/TCE, PARADE/PDE, HIGHWAY/HWY, ESPLANADE/ESP, BOULEVARD/BVD, LANE, WAY, CLOSE/CL, and 200+ more.

### Unit Prefixes
UNIT, FLAT, APARTMENT/APT, VILLA, LOT, SHOP, SUITE, ROOM, OFFICE, FACTORY, WAREHOUSE, SHED, KIOSK, TOWNHOUSE, PENTHOUSE, STUDIO, and more.

### Level Prefixes
LEVEL/L, FLOOR/FL, GROUND/G, BASEMENT/B, MEZZANINE/M, LOWER GROUND/LG, UPPER GROUND/UG, PODIUM, ROOFTOP, and more.

### Special Delivery Types
PO BOX, GPO BOX, LOCKED BAG, PRIVATE BAG, REPLY PAID, RMB, CMB, RSD, MS, CMA, CPA, CARE PO

## G-NAF Test Data

The parser is validated against real addresses from G-NAF. To regenerate the test dataset:

1. Download G-NAF from [data.gov.au](https://data.gov.au/data/dataset/geocoded-national-address-file-g-naf)
2. Extract to the repository root
3. Run the generator:

```bash
go build ./cmd/gnaf-gen
./gnaf-gen -gnaf "g-naf_nov25_allstates_gda94_psv_1021/G-NAF/G-NAF NOVEMBER 2025/Standard" \
    -output testdata/gnaf_addresses.json \
    -count 10000
```

The G-NAF dataset is excluded from git via `.gitignore`.

## Development and Releases

The repository includes a Taskfile for routine maintenance:

```bash
task test
task gnaf:update
task release VERSION=v1.2.3
task release:push VERSION=v1.2.3
```

`gnaf:update` resolves the latest GDA2020 G-NAF ZIP from the official
data.gov.au package metadata, regenerates the embedded locality index, and runs
the verification suite. `release` requires a clean worktree and creates only a
local annotated semantic-version tag; pushing that tag is a separate explicit
step.

## References

- [Australia Post Addressing Guidelines](https://auspost.com.au/sending/guidelines/addressing-guidelines)
- [Australia Post Addressing Standards](https://auspost.com.au/content/dam/auspost_corp/media/documents/australia-post-addressing-standards-1999.pdf)
- [AMAS Developer Guide](https://auspost.com.au/content/dam/auspost_corp/media/documents/amas-developer-guide.pdf)
- [G-NAF - Geocoded National Address File](https://data.gov.au/data/dataset/geocoded-national-address-file-g-naf)

## License

MIT

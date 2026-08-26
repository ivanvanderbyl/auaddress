# parse-address command design

## Goal

Add a Go-installable `parse-address` command for normalising one Australian
address or comparing two addresses. Keep the parser package as the source of
truth and use the command only for argument handling and presentation.

## Command interface

Parsing is the root action:

```sh
parse-address 'Level 4, 54 Wellington Street, Collingwood'
parse-address 'Level 4, 54 Wellington Street, Collingwood' --json
```

The default output is the address returned by `ParsedAddress.Format()`:

```text
L 4 54 WELLINGTON ST
COLLINGWOOD
```

Comparison is a subcommand:

```sh
parse-address compare ADDRESS_A ADDRESS_B
parse-address compare ADDRESS_A ADDRESS_B --json
```

The plain comparison output begins with `exact`, `partial`, or `no match`, then
prints matched and missing component details when they explain the result.

Use `github.com/urfave/cli/v3` version `v3.11.0`. Version 3 permits flags after
positional arguments, so `--json` works in the requested position. Do not add a
redundant `parse` subcommand, standard-input mode, batch parsing, or a
configuration file in this version.

## Parsing and exit behaviour

Both commands use a strict `auaddress.Parser`. A missing or invalid address
returns a non-zero exit status and writes an error to standard error. The
comparison command identifies whether the left or right input failed. A valid
`NoMatch` comparison remains a successful command result.

Keep the command testable without a subprocess. Construct the urfave command
with injected output streams, and put process exit handling in a small `main`
function.

## JSON output

Use purpose-built command output types rather than serialising `ParsedAddress`
or `AddressMatch` directly. This keeps compatibility fields and Go enum values
out of the command contract.

Address JSON uses camelCase field names and ordered delivery points:

```json
{
  "deliveryPoints": [
    {
      "kind": "street",
      "level": "L 4",
      "streetNumber": "54",
      "streetName": "WELLINGTON",
      "streetType": "ST"
    }
  ],
  "locality": "COLLINGWOOD"
}
```

Street points may contain `unit`, `level`, `streetNumber`, `streetName`,
`streetType`, and `streetSuffix`. Postal points contain `kind`, `postalType`,
and `postalNumber`. Empty optional fields are omitted. `nameLines` appears only
when recipient or organisation lines are present.

Comparison JSON uses string values for the comparison kind and components:

```json
{
  "kind": "partial",
  "matchedThrough": "locality",
  "missingFromLeft": ["state", "postcode"],
  "missingFromRight": [],
  "leftKey": "...",
  "rightKey": "..."
}
```

With `--json`, command failures write a single `{"error":"..."}` object to
standard error before exiting non-zero.

## Testing

Table-driven command tests cover formatted parse output, address JSON, flag
placement before and after the address, postal delivery JSON, exact and partial
comparison text, comparison JSON, missing arguments, excess arguments, and
invalid left and right addresses. Repository verification still runs formatting,
`go vet`, and all Go tests.

# Multi-address CLI design

## Goal

Make the default `parse-address` action parse every independently completed
address in its single input argument. The command has not shipped, so the
initial public output contract can be consistently multi-address rather than
preserving the provisional single-address shape.

## Command interface

The root action continues to accept exactly one positional input string:

```sh
parse-address '54 Wellington St, Collingwood\nPO Box 234, Melbourne'
parse-address '54 Wellington St, Collingwood\nPO Box 234, Melbourne' --json
```

At the CLI boundary, literal `\n` sequences become newline characters. Actual
newline characters remain supported. This makes multiline shell input usable
without requiring shell-specific quoting.

The root action always calls strict `ParseAll`. There is no `--all` flag,
automatic scalar-or-array switching, or separate parsing subcommand.
`compare` remains unchanged.

## Output

Plain output formats every parsed address with `ParsedAddress.Format()` and
places one blank line between addresses:

```text
54 WELLINGTON ST
COLLINGWOOD

PO BOX 234
MELBOURNE
```

JSON output is always an array of the existing camelCase address objects, even
when the input contains exactly one address:

```json
[
  {
    "deliveryPoints": [
      {
        "kind": "street",
        "streetNumber": "54",
        "streetName": "WELLINGTON",
        "streetType": "ST"
      }
    ],
    "locality": "COLLINGWOOD"
  },
  {
    "deliveryPoints": [
      {
        "kind": "postal",
        "postalType": "PO BOX",
        "postalNumber": "234"
      }
    ],
    "locality": "MELBOURNE"
  }
]
```

The command uses compact JSON encoding, as it does today. The expanded example
only illustrates the structure.

## Failures

Strict parsing remains the command default. If any part of the input fails,
the command writes the existing plain or JSON error to standard error, exits
non-zero, and writes no partial result to standard output.

## Testing and documentation

Command tests cover one and multiple addresses in plain and JSON modes, both
actual and escaped newlines, the stable one-element JSON array, and failure
without partial output. Comparison tests remain unchanged.

The README quickstart uses the JSON array contract and adds a multiline CLI
example. The Go `ParseAll` section remains the deeper library explanation.

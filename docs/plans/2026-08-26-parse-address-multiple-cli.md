# Multi-address CLI implementation record

**Completed:** 26 August 2026

## Goal

Make the default `parse-address` action parse and format every address in one
input string, with predictable output for both single and multiple addresses.

## Architecture

Multi-address parsing remains in the root `auaddress` package. The CLI calls
strict `Parser.ParseAll`, converts results through its command-owned JSON
model, and buffers plain output before writing it. Comparison remains a
separate single-address path.

The root action accepts actual newlines and converts literal `\n` sequences to
newlines at the command boundary. `parseAllStrict` only constructs a strict
parser and calls `ParseAll`; comparison continues to use `parseStrict`.

## Implemented behaviour

- The root action parses every independently completed address in one
  positional input.
- JSON output is always an array, including when the input contains one
  address.
- Multiple plain-text results are separated by one blank line.
- Results preserve their encounter order.
- `compare` remains unchanged and its JSON output remains an object.
- Parsing is atomic for both plain and JSON output. If any address is invalid,
  the command exits nonzero and emits no partial standard output.

For example:

```bash
go run ./cmd/parse-address 'Level 4, 54 Wellington St, Collingwood\nPO Box 234, Melbourne' --json
```

returns two address objects: the Wellington Street address in `COLLINGWOOD`,
followed by `PO BOX 234` in `MELBOURNE`.

## Implementation evidence

- `ece8d0d Parse multiple addresses by default` added the root `ParseAll`
  path, root-only literal newline conversion, JSON arrays, and blank-line plain
  formatting. The focused tests were observed failing before implementation
  and passing afterward.
- `02db025 Test atomic multi-address failures` added plain and JSON regression
  coverage proving that invalid multi-address input produces no partial
  standard output.
- `0f322495 Document multi-address command output` updated the README examples
  and command contract.

## Verification evidence

The completed implementation passed:

```bash
task test
go test -count=1 ./...
git diff --check
```

Command-level verification also confirmed:

- The exact reported command above returns a two-element JSON array ordered
  `COLLINGWOOD`, then `MELBOURNE`, with the expected street and postal fields.
- Single-address JSON is a one-element array.
- Plain multi-address output contains one blank line between addresses.
- Compare JSON remains an object.
- Invalid multi-address input in both plain and JSON modes exits nonzero with
  zero bytes written to standard output.
- `L4 54 Wellington Street, Collingwood` compares exactly with
  `Level 4, 54 Wellington Street, Collingwood`.

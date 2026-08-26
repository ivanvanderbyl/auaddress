# parse-address CLI and README implementation plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a tested, Go-installable `parse-address` command and rewrite the README around immediate command-line and library success.

**Architecture:** Keep all address parsing and comparison behaviour in the root `auaddress` package. Add a thin urfave/cli v3 command that validates positional arguments, uses strict parsing, converts public results into stable camelCase output models, and writes either human-readable text or JSON. Document the binary first, the Go API second, and the locality-terminated grammar after both quickstarts.

**Tech Stack:** Go 1.25.5, `github.com/urfave/cli/v3` v3.11.0, `encoding/json`, Task, Markdown.

---

### Task 1: Establish the baseline and add urfave/cli

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Run the existing test suite**

Run:

```bash
task test
```

Expected: PASS before the command package is added.

**Step 2: Add the pinned command dependency**

Run:

```bash
go get github.com/urfave/cli/v3@v3.11.0
```

Expected: `go.mod` and `go.sum` record urfave/cli v3.11.0.

**Step 3: Commit the dependency**

```bash
git add go.mod go.sum
git commit -m "Add urfave CLI dependency"
```

### Task 2: Add the default parse command

**Files:**
- Create: `cmd/parse-address/main.go`
- Create: `cmd/parse-address/command.go`
- Create: `cmd/parse-address/command_test.go`

**Step 1: Write failing parse command tests**

Add table-driven tests around a helper with this contract:

```go
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int
```

Cover:

```go
{
    name: "formatted address",
    args: []string{"parse-address", "Level 4, 54 Wellington Street, Collingwood"},
    stdout: "L 4 54 WELLINGTON ST\nCOLLINGWOOD\n",
    code: 0,
}
```

Also assert that missing and excess address arguments return a non-zero code
and write a useful error to standard error.

**Step 2: Run the focused test and verify it fails**

Run:

```bash
go test ./cmd/parse-address -run TestRunParse -v
```

Expected: FAIL because `run` does not exist.

**Step 3: Implement the command shell**

Build a root `cli.Command` with:

```go
&cli.Command{
    Name:      "parse-address",
    Usage:     "parse and compare Australian addresses",
    ArgsUsage: "ADDRESS",
    Action:    parseAction(stdout),
}
```

`parseAction` requires exactly one positional argument, parses it with
`auaddress.NewParser(auaddress.WithStrict(true))`, and writes
`address.Format()` plus a trailing newline.

Keep `main` limited to:

```go
func main() {
    os.Exit(run(context.Background(), os.Args, os.Stdout, os.Stderr))
}
```

**Step 4: Format and run the focused tests**

Run:

```bash
gofmt -w cmd/parse-address/*.go
go test ./cmd/parse-address -run TestRunParse -v
```

Expected: PASS.

**Step 5: Commit the parse command**

```bash
git add cmd/parse-address
git commit -m "Add parse-address command"
```

### Task 3: Add camelCase address JSON

**Files:**
- Modify: `cmd/parse-address/command.go`
- Modify: `cmd/parse-address/command_test.go`

**Step 1: Write failing JSON tests**

Test `--json` before and after the address. Decode the output and compare it to
this command-owned model:

```go
type addressOutput struct {
    DeliveryPoints []deliveryPointOutput `json:"deliveryPoints"`
    NameLines      []string              `json:"nameLines,omitempty"`
    Locality       string                `json:"locality"`
    State          string                `json:"state,omitempty"`
    Postcode       string                `json:"postcode,omitempty"`
}

type deliveryPointOutput struct {
    Kind          string `json:"kind"`
    Unit          string `json:"unit,omitempty"`
    Level         string `json:"level,omitempty"`
    StreetNumber  string `json:"streetNumber,omitempty"`
    StreetName    string `json:"streetName,omitempty"`
    StreetType    string `json:"streetType,omitempty"`
    StreetSuffix  string `json:"streetSuffix,omitempty"`
    PostalType    string `json:"postalType,omitempty"`
    PostalNumber  string `json:"postalNumber,omitempty"`
}
```

Add a postal case for `PO Box 42, Richmond`.

**Step 2: Run the JSON tests and verify they fail**

Run:

```bash
go test ./cmd/parse-address -run 'TestRunParseJSON|TestAddressOutput' -v
```

Expected: FAIL because the flag and output models do not exist.

**Step 3: Implement JSON conversion and encoding**

Add a `--json` `cli.BoolFlag`, convert canonical `DeliveryPoints` in encounter
order, and encode with `json.NewEncoder(stdout)`. Do not serialise
`ParsedAddress` directly.

**Step 4: Format and run the JSON tests**

Run:

```bash
gofmt -w cmd/parse-address/*.go
go test ./cmd/parse-address -run 'TestRunParseJSON|TestAddressOutput' -v
```

Expected: PASS.

**Step 5: Commit JSON parsing output**

```bash
git add cmd/parse-address
git commit -m "Add parse-address JSON output"
```

### Task 4: Add the compare subcommand

**Files:**
- Modify: `cmd/parse-address/command.go`
- Modify: `cmd/parse-address/command_test.go`

**Step 1: Write failing comparison tests**

Cover exact, partial, and no-match plain output. The partial fixture is:

```text
left:  54 Wellington Street, Collingwood
right: 54 Wellington St, Collingwood VIC 3066
```

Expected plain output:

```text
partial
matched through: locality
missing from left: state, postcode
```

Test comparison JSON with `kind`, `matchedThrough`, both missing-component
arrays, and both comparison keys. Test `--json` after both positional arguments.

**Step 2: Run the comparison tests and verify they fail**

Run:

```bash
go test ./cmd/parse-address -run TestRunCompare -v
```

Expected: FAIL because the subcommand does not exist.

**Step 3: Implement comparison presentation**

Add a `compare` child command requiring exactly two addresses. Parse both with
the strict parser, call `auaddress.CompareAddresses`, and convert enums with
exhaustive command-local functions:

```go
func matchKindName(kind auaddress.MatchKind) string
func matchComponentName(component auaddress.MatchComponent) string
```

Use this JSON type:

```go
type comparisonOutput struct {
    Kind             string   `json:"kind"`
    MatchedThrough   string   `json:"matchedThrough,omitempty"`
    MissingFromLeft  []string `json:"missingFromLeft"`
    MissingFromRight []string `json:"missingFromRight"`
    LeftKey          string   `json:"leftKey"`
    RightKey         string   `json:"rightKey"`
}
```

A valid no-match result exits zero.

**Step 4: Format and run comparison tests**

Run:

```bash
gofmt -w cmd/parse-address/*.go
go test ./cmd/parse-address -run TestRunCompare -v
```

Expected: PASS.

**Step 5: Commit comparison support**

```bash
git add cmd/parse-address
git commit -m "Add address comparison command"
```

### Task 5: Make failures machine-readable

**Files:**
- Modify: `cmd/parse-address/command.go`
- Modify: `cmd/parse-address/command_test.go`

**Step 1: Write failing error-output tests**

Cover invalid parse input, invalid left comparison input, and invalid right
comparison input. With `--json`, expect one standard-error object:

```json
{"error":"invalid address format"}
```

Plain failures remain readable text. All failures return non-zero and leave
standard output empty.

**Step 2: Run the error tests and verify they fail**

Run:

```bash
go test ./cmd/parse-address -run 'TestRunErrors|TestRunJSONErrors' -v
```

Expected: FAIL until `run` centralises error presentation.

**Step 3: Implement error presentation**

Track the selected JSON mode through the command invocation. On failure, write
either `err.Error()` or a camelCase-independent `error` JSON object to standard
error and return exit code 1. Wrap comparison failures with `left address` or
`right address` before presentation.

**Step 4: Format and run all command tests**

Run:

```bash
gofmt -w cmd/parse-address/*.go
go test ./cmd/parse-address -v
```

Expected: PASS.

**Step 5: Commit error handling**

```bash
git add cmd/parse-address
git commit -m "Report parse-address failures"
```

### Task 6: Rewrite the README reader path

**Files:**
- Modify: `README.md`

**Step 1: Replace the opening with the outcome and CLI quickstart**

Use @editorial-voice in edit mode. Open with this claim:

```markdown
Parse complete or partial Australian addresses from Go or the command line.
```

Show:

```bash
go install github.com/ivanvanderbyl/auaddress/cmd/parse-address@latest
parse-address 'Level 4, 54 Wellington Street, Collingwood'
```

and its two-line normalised output.

**Step 2: Add the Go library quickstart**

Show `go get`, then a complete `package main` example parsing
`3A/45 High Street, Richmond` and printing its normalised unit, delivery line,
and locality.

**Step 3: Explain the grammar after both quickstarts**

Show:

```text
Address := DeliveryPoint+ Locality State? Postcode?
DeliveryPoint := StreetDelivery | PostalDelivery
```

Explain greedy left-to-right parsing, soft comma and newline boundaries,
locality termination, and ordered delivery points. Keep the explanation at the
public address-input boundary.

**Step 4: Retain the approved progressive sequence**

Order the remaining verified material as:

1. JSON command output and comparison command.
2. `ParseAll` and shared delivery points.
3. Go comparison API.
4. Strict and lenient parsing failures.
5. Supported components.
6. Locality data.
7. Scope and limitations in public language.
8. Development, release, references, and licence.

Remove the full public type listing. Keep a short compatibility note for
`DeliveryPoints` and the flat fields.

**Step 5: Run editorial checks**

Confirm sentence-case headings, Australian English, no em dashes, no unsupported
performance or validation claims, and no internal address-tag terminology.

### Task 7: Verify the published commands and repository

**Files:**
- Verify: `README.md`
- Verify: `cmd/parse-address/*`
- Add: `docs/plans/2026-08-26-parse-address-cli-and-readme.md`

**Step 1: Install and run the command from a temporary binary directory**

Run the documented `go install` command with `GOBIN` set to a directory created
by `mktemp -d`, then run the documented parse, JSON, compare, and compare-JSON
examples.

Expected: output matches the README byte for byte apart from JSON indentation
where the README abbreviates long comparison keys.

**Step 2: Check formatting and tests**

Run:

```bash
gofmt -w cmd/parse-address/*.go
git diff --check
task test
```

Expected: all commands succeed.

**Step 3: Check the dependency version**

Run:

```bash
go list -m github.com/urfave/cli/v3
```

Expected: the module command prints `github.com/urfave/cli/v3 v3.11.0`.

**Step 4: Inspect the final diff**

Run:

```bash
git diff --stat HEAD~5..HEAD
git status --short
```

Expected: only the command, dependency, README, and approved plan documents are
changed.

**Step 5: Commit the documentation**

```bash
git add README.md docs/plans/2026-08-26-parse-address-cli-and-readme.md
git commit -m "Rewrite README for Go developers"
```

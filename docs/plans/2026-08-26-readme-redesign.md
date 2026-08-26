# README redesign

## Reader and outcome

The README serves Go developers deciding whether `auaddress` fits their
Australian address parsing problem. A reader who stops after the first screen
should know what the package returns, what completes an address, how to install
it, and where its scope ends.

## Opening

Lead with the outcome: parse complete or partial Australian addresses into
structured Go values. Follow it with a runnable example using
`3A/45 High Street, Richmond` and show the normalised unit, street, and locality
fields.

State the minimum grammar contract next to the example. A locality completes an
address. State and postcode add specificity but remain optional. Explain in the
limitations that the package parses supplied address text rather than locating
an address inside surrounding prose.

## Structure

Lead with the installable `parse-address` command because it produces the
fastest visible result. Follow it immediately with the Go library quickstart.
After those first successful parses, explain the library in this order:

1. Show the compact address grammar and explain locality termination, soft
   comma and newline boundaries, and ordered delivery points.
2. Demonstrate `ParseAll`, including the difference between independently
   locality-terminated addresses and delivery points sharing one locality.
3. Demonstrate exact, partial, and conflicting component comparison.
4. Explain lenient and strict error handling.
5. Summarise supported address components.
6. Explain the embedded Geocoded National Address File locality index.
7. State the extraction, existence-validation, postcode-validation,
   international-address, and certification limits.
8. Keep development, release, reference, and licence material at the end.

Do not include a full public type listing in the main path. The examples should
introduce the fields a new user needs, while Go package documentation remains
the complete API reference.

## Voice and evidence

Use a task-first documentation voice. Prefer realistic inputs and observable
outputs over feature claims. Keep grammar terminology where it predicts parser
behaviour, but introduce each term after the first parse succeeds.

Retain only claims supported by the implementation, tests, generated locality
data, or existing external references. Put limitations beside the relevant
contract rather than presenting the parser as a general-purpose address
extractor or authoritative address validator.

## Verification

Run every README code example against the current public API, confirm all
commands and counts against the checkout, inspect the rendered heading order,
and run the repository test task before completion.

# Release every merged pull request as a patch

## Decision

Every pull request merged into `main` creates the next stable patch release.
The repository currently has `v0.0.1`, so the next merged pull request creates
`v0.0.2`.

This follows the release flow in Alcova's `adk-anthropic-go` repository: show a
tag preview while a pull request is open, then tag the merge commit and publish
a GitHub Release when it merges. This project adds concurrency and retry checks
so two merges cannot claim the same version and a rerun cannot advance the same
merge twice.

## Version calculation

Add `scripts/gh/determine_highest_version_prefix.py`. It reads repository tags
matching `vMAJOR.MINOR.PATCH`, selects the highest semantic version, and
increments its patch component. Tags outside that stable format do not affect
the result. A repository without a stable tag starts at `v0.1.0`, matching the
source project.

Keep the calculator independent from GitHub so local unit tests can cover tag
filtering, semantic ordering, patch increments, large numeric components, and
the no-tag fallback. `task test` runs those tests alongside the Go checks.

## Workflow

Add `.github/workflows/tag-merged-pr.yml` with `contents: write` permission and
a `release-main` concurrency group.

For an open, reopened, or updated pull request targeting `main`, the preview
job fetches the full tag history, calculates the next patch tag, and writes it
to the workflow summary. It does not create repository state.

For a merged pull request, the release job checks out the merge commit and
fetches all tags. It first looks for an existing stable tag pointing at that
commit. A retry reuses that tag. A new merge calculates the next patch version,
creates an annotated tag, and pushes it.

The final step creates a GitHub Release with generated notes since the previous
stable tag. If the release already exists, the step exits successfully. The
workflow never writes a version file because a Go module's release version is
its Git tag.

## Existing manual release path

Keep `task release VERSION=vX.Y.Z` and `task release:push VERSION=vX.Y.Z` as an
explicit recovery path. The automated workflow owns ordinary merged-pull-
request releases. Both paths use the same stable tag format.

## Failures and boundaries

The workflow fails before tagging when no stable ancestor tag exists, version
calculation fails, or the next tag already points elsewhere. A failed GitHub
Release creation can be retried without creating another version because the
merge commit already carries its tag.

This change does not build binary archives, publish a Homebrew formula, infer
minor or major releases, or add an embedded binary version. Those can be added
when the distribution path needs them.

## Verification

Tests exercise version calculation without calling GitHub. Workflow review
checks event filters, permissions, concurrency, tag ancestry, retry behaviour,
and generated-release arguments. Repository verification still runs
`task test` and `go test -count=1 ./...`.

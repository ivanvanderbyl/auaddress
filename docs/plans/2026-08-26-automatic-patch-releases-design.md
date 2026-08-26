# Release every merged pull request as a patch

## Decision

Every pull request merged into `main` creates the next stable patch release.
The repository currently has `v0.0.1`, so the next merged pull request creates
`v0.0.2`.

This follows the release flow in Alcova's `adk-anthropic-go` repository: show a
tag preview while a pull request is open, then tag the merge commit and publish
a GitHub Release when it merges. This project separates those stages across a
read-only preview and closure workflows and a privileged `workflow_run`
reconciler. The reconciler orders releases by `main`'s first-parent history, so workflow arrival
order cannot reorder versions and a rerun cannot advance the same merge twice.

## Version calculation

Add `scripts/gh/determine_highest_version_prefix.py`. It reads repository tags
matching `vMAJOR.MINOR.PATCH`, selects the highest semantic version, and
increments its patch component. Tags outside that stable format do not affect
the result. A repository without a stable tag starts at `v0.1.0`, matching the
source project.

Tag parsing uses Git's newline-delimited output without trimming candidates.
The stable grammar accepts ASCII digits without leading zeroes, so malformed or
Unicode-whitespace-suffixed tags are ignored consistently by the calculator and
reconciler.

Keep the calculator independent from GitHub so local unit tests can cover tag
filtering, semantic ordering, patch increments, large numeric components, and
the no-tag fallback. `task test` runs those tests alongside the Go checks.

## Workflow

`.github/workflows/tag-merged-pr.yml` handles only open, reopened, and updated
pull requests with read-only contents permission. It checks out the trusted
base commit without persisted credentials, calculates the next patch tag, and
writes it to the workflow summary. No privileged workflow watches previews.

`.github/workflows/observe-merged-pr.yml` handles only closed pull requests. It
does not check out code or write repository state, and gives each run a trusted
numeric PR identity. `.github/workflows/release-merged-prs.yml` watches only
that observer. Its reconciliation job alone receives contents-write permission.
It checks out trusted default-branch code, does not consume upstream artifacts
or caches, and uses the GitHub API to bind the run identity and metadata to a
pull request merged into `main`.

Starting at the latest contiguous published stable release, or the existing
`v0.0.1` seed, the reconciler walks only the unreleased interval of `main` in
first-parent order. Merge commits, squash commits, and the last commit of a
rebased pull request are release points; direct pushes are not. It reconciles
every missing release point through the triggering pull request, creates each
annotated patch tag in ancestry order, and publishes generated notes since the
preceding tag.
The workflow never writes a version file because a Go module's version is its
Git tag.

## Existing manual release path

Keep `task release VERSION=vX.Y.Z` and `task release:push VERSION=vX.Y.Z` as an
explicit recovery path. The automated workflow owns ordinary merged-pull-
request releases. Both paths use the same stable tag format.

A manually pushed stable tag on a verified release point may jump forward to a
higher version, such as `v1.2.3`. Reconciliation publishes its missing GitHub
Release and then continues automatic patch releases at `v1.2.4`. `task release`
alone creates only a local tag; it does not publish a tag or GitHub Release.

## Failures and boundaries

The privileged job no-ops for open and closed-unmerged pull requests. It fails
before tagging when the upstream run lacks one verifiable pull request, the
`v0.0.1` seed is invalid, a release point cannot be verified, or stable tags
conflict with ancestry order. A failed GitHub Release creation can be retried:
an existing correct tag with no Release creates only the missing Release, while
a completed tag-and-Release pair is a no-op. The seed tag itself is never
retroactively published as a GitHub Release.

This change does not build binary archives, publish a Homebrew formula, infer
minor or major releases, or add an embedded binary version. Those can be added
when the distribution path needs them.

## Verification

Tests exercise exact version parsing and ordered reconciliation with pure
planner cases, mocked Git/GitHub clients, pagination and failure boundaries, and
a temporary real Git repository. They cover reversed event arrival, bounded
anchor recovery, manual version jumps, draft/prerelease states, direct pushes,
conflicts, and merge, squash, and rebase release points. Workflow review checks
the three-stage trust boundary, minimal permissions, immutable action pins,
release-only queueing, and the absence of upstream artifacts and caches.
Repository verification still runs `task test` and `go test -count=1 ./...`.

# Automatic patch releases implementation plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create the next patch tag and GitHub Release whenever a pull request merges into `main`.

**Architecture:** Adapt Alcova's `adk-anthropic-go` merged-pull-request workflow to this single Go module. Keep semantic-version calculation in a small tested Python script, preview the next tag while a pull request is open, and make the merge path concurrency-safe and idempotent before it pushes an annotated tag. Preserve the existing Task-based manual release path.

**Tech Stack:** GitHub Actions, Bash, Python 3 standard library, Git, GitHub CLI, Task, Go 1.25.5.

---

### Task 1: Add the tested patch-version calculator

**Files:**
- Create: `scripts/gh/determine_highest_version_prefix.py`
- Create: `scripts/gh/test_determine_highest_version_prefix.py`
- Modify: `Taskfile.yml`

**Step 1: Write failing calculator tests**

Create a standard-library `unittest` suite that imports `next_version` and covers:

```python
class NextVersionTests(unittest.TestCase):
    def test_increments_highest_stable_patch(self):
        self.assertEqual(
            next_version(["v0.0.9", "v0.0.10", "v0.1.0"]),
            (0, 1, 1),
        )

    def test_ignores_non_stable_tags(self):
        self.assertEqual(
            next_version(["v1.2.3-rc.1", "go/v9.0.0", "v1.2.3"]),
            (1, 2, 4),
        )

    def test_starts_at_v0_1_0_without_stable_tags(self):
        self.assertEqual(next_version([]), (0, 1, 0))

    def test_supports_large_components(self):
        self.assertEqual(next_version(["v12.99.999"]), (12, 99, 1000))
```

Also test that `main()` prints `MAJOR.MINOR.PATCH` and appends
`version=MAJOR.MINOR.PATCH` to the path named by `GITHUB_OUTPUT`.

**Step 2: Run the tests and verify RED**

Run:

```bash
python3 -m unittest discover -s scripts/gh -p 'test_*.py' -v
```

Expected: FAIL because `determine_highest_version_prefix.py` does not exist.

**Step 3: Implement the calculator**

Adapt the `adk-anthropic-go` script with these public functions:

```python
TAG_PATTERN = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")
FALLBACK_VERSION = (0, 1, 0)

def git_tags() -> Iterable[str]: ...
def next_version(tags: Iterable[str]) -> Tuple[int, int, int]: ...
def write_output(version: str) -> None: ...
def main() -> int: ...
```

`next_version` parses integers and uses tuple ordering, not string ordering.
`main` reads the current repository tags, prints the next version without the
leading `v`, and writes the GitHub Actions output when available.

**Step 4: Run the calculator tests and verify GREEN**

Run:

```bash
python3 -m unittest discover -s scripts/gh -p 'test_*.py' -v
```

Expected: all calculator and command tests pass.

**Step 5: Add calculator tests to the repository test task**

Add this command to `Taskfile.yml` after Go tests:

```yaml
- python3 -m unittest discover -s scripts/gh -p 'test_*.py'
```

Run `task test`. Expected: formatting, vet, Go tests, and calculator tests pass.

**Step 6: Commit the calculator**

```bash
git add Taskfile.yml scripts/gh
git commit -m "Add release version calculator"
```

### Task 2: Add the merged-pull-request release workflow

**Files:**
- Create: `.github/workflows/tag-merged-pr.yml`

**Step 1: Create the workflow trigger and permissions**

Start with:

```yaml
name: Tag merged PRs

on:
  pull_request:
    branches: [main]
    types: [opened, reopened, synchronize, closed]

permissions:
  contents: write

concurrency:
  group: release-main
  cancel-in-progress: false
```

**Step 2: Add the preview job**

Run the preview only while the pull request is open:

```yaml
preview-pr-tag:
  if: github.event.action != 'closed'
```

Check out full history with tags, fetch tags explicitly, run the version
calculator with step id `next-version`, and append this to
`$GITHUB_STEP_SUMMARY`:

```text
### Release tag preview

If merged, this pull request will be tagged as `v${version}`.
```

**Step 3: Add idempotent merge-version calculation**

Run the release job only when `github.event.pull_request.merged == true`.
Check out `github.event.pull_request.merge_commit_sha` with full history.

The calculation step must:

1. Fetch all tags.
2. Find an existing stable tag pointing at the merge commit.
3. Reuse that tag on a retry and set `create_tag=false`.
4. Otherwise find the latest stable ancestor tag, fail if none exists, run the
   calculator, prefix its result with `v`, and set `create_tag=true`.
5. Emit `previous_tag`, `next_tag`, and `create_tag` outputs.

Use the stable pattern:

```regex
^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$
```

**Step 4: Tag the merge commit**

When `create_tag == 'true'`, configure the GitHub Actions bot identity, create
an annotated tag at `MERGE_SHA`, and push only that tag ref:

```bash
git tag -a "$NEXT_TAG" "$MERGE_SHA" -m "Release $NEXT_TAG"
git push origin "refs/tags/$NEXT_TAG"
```

Before creating it, fail if the calculated tag exists locally or remotely.

**Step 5: Publish an idempotent GitHub Release**

Use `GH_TOKEN: ${{ github.token }}`. If `gh release view "$NEXT_TAG"` succeeds,
exit zero. Otherwise run:

```bash
gh release create "$NEXT_TAG" \
  --verify-tag \
  --title "$NEXT_TAG" \
  --generate-notes \
  --notes-start-tag "$PREVIOUS_TAG"
```

**Step 6: Validate the workflow locally**

Run:

```bash
actionlint .github/workflows/tag-merged-pr.yml
```

Expected: no diagnostics. `actionlint` also invokes the installed ShellCheck
for embedded Bash.

**Step 7: Review mutation boundaries**

Confirm:

- only the merged job has a tag or release mutation path;
- the preview job cannot write repository state;
- the concurrency group serialises releases;
- the merged job checks for a tag on its merge commit before calculating;
- only stable root-module tags affect the next version;
- a repeated run reuses the existing tag and release.

**Step 8: Commit the workflow**

```bash
git add .github/workflows/tag-merged-pr.yml
git commit -m "Release every merged pull request"
```

### Task 3: Document automatic and manual releases

**Files:**
- Modify: `README.md`

**Step 1: Update the release instructions**

In `Development and releases`, state:

- every pull request merged into `main` creates the next patch tag and GitHub
  Release;
- open pull requests show the proposed tag in the workflow summary;
- `release` and `release:push` remain the manual recovery path.

Do not claim that the workflow builds binary archives, publishes Homebrew, or
updates a version file.

**Step 2: Update the test description**

State that `task test` runs the release-version calculator tests as well as Go
formatting, vet, and tests.

**Step 3: Run the documentation checks**

Run:

```bash
git diff --check
rg -n 'merged|patch|GitHub Release|manual' README.md
```

Expected: the automatic path and manual fallback are both explicit.

**Step 4: Commit the README**

```bash
git add README.md
git commit -m "Document automatic patch releases"
```

### Task 4: Verify the completed release path

**Files:**
- Verify: `.github/workflows/tag-merged-pr.yml`
- Verify: `scripts/gh/determine_highest_version_prefix.py`
- Verify: `Taskfile.yml`
- Verify: `README.md`

**Step 1: Run all local verification**

Run:

```bash
task test
go test -count=1 ./...
python3 -m unittest discover -s scripts/gh -p 'test_*.py' -v
actionlint .github/workflows/tag-merged-pr.yml
git diff --check
```

Expected: all commands succeed.

**Step 2: Verify the current next version**

Run:

```bash
python3 scripts/gh/determine_highest_version_prefix.py
```

Expected: `0.0.2`, because `v0.0.1` is the highest stable tag.

**Step 3: Inspect the final branch**

Run:

```bash
git status --short
git log -6 --oneline
```

Expected: a clean worktree with calculator, workflow, and documentation commits
after the approved design and implementation-plan commits.

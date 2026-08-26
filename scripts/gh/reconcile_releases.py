#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import re
import subprocess
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

from determine_highest_version_prefix import TAG_PATTERN


class ReleaseError(RuntimeError):
    pass


@dataclass(frozen=True)
class PullRequest:
    number: int
    state: str
    merged: bool
    base_ref: str
    base_repository: str
    merge_commit_sha: str | None
    head_ref: str | None = None
    head_sha: str | None = None
    head_repository: str | None = None
    base_sha: str | None = None


@dataclass(frozen=True)
class ReleaseAction:
    pull_request: int
    commit: str
    previous_tag: str
    next_tag: str
    create_tag: bool
    create_release: bool
    publish_draft: bool = False


@dataclass(frozen=True)
class ReleaseState:
    draft: bool = False
    prerelease: bool = False


RUN_NAME_PATTERN = re.compile(r"^Release PR #([1-9][0-9]*)$")


def version_tuple(tag: str) -> tuple[int, int, int] | None:
    match = TAG_PATTERN.fullmatch(tag)
    if not match:
        return None
    return tuple(map(int, match.groups()))


def select_completed_anchor(
    *,
    seed_tag: str,
    seed_commit: str,
    main_commits: Sequence[str],
    tags: Mapping[str, str],
    releases: Mapping[str, ReleaseState],
) -> tuple[str, str]:
    seed_version = version_tuple(seed_tag)
    if seed_version is None or tags.get(seed_tag) != seed_commit:
        raise ReleaseError(f"Invalid release seed {seed_tag} at {seed_commit}.")
    positions = {seed_commit: 0}
    positions.update({commit: index + 1 for index, commit in enumerate(main_commits)})
    ordered_tags: list[tuple[int, tuple[int, int, int], str, str]] = []
    tags_at_commit: dict[str, list[str]] = {}
    for tag, commit in tags.items():
        version = version_tuple(tag)
        if version is None or version <= seed_version:
            continue
        if commit not in positions:
            raise ReleaseError(f"Stable tag {tag} is outside ordered main ancestry.")
        tags_at_commit.setdefault(commit, []).append(tag)
        ordered_tags.append((positions[commit], version, tag, commit))
    for commit, commit_tags in tags_at_commit.items():
        if len(commit_tags) > 1:
            raise ReleaseError(
                f"Multiple stable tags point at main commit {commit}: "
                f"{', '.join(sorted(commit_tags))}."
            )

    anchor_tag, anchor_commit = seed_tag, seed_commit
    previous_version = seed_version
    incomplete_seen = False
    for _, version, tag, commit in sorted(ordered_tags):
        if version <= previous_version:
            raise ReleaseError(
                f"Stable tag {tag} does not increase after the preceding release."
            )
        previous_version = version
        state = releases.get(tag)
        if state and state.prerelease:
            raise ReleaseError(
                f"Stable tag {tag} has a prerelease GitHub Release; promote or "
                "remove it before retrying."
            )
        if state and not state.draft:
            if incomplete_seen:
                raise ReleaseError(
                    f"Published release {tag} follows an incomplete earlier release."
                )
            anchor_tag, anchor_commit = tag, commit
        else:
            incomplete_seen = True
    return anchor_tag, anchor_commit


def plan_releases(
    *,
    seed_tag: str,
    seed_commit: str,
    commits: Sequence[str],
    pull_requests_by_commit: Mapping[str, Sequence[PullRequest]],
    target_pull_request: PullRequest,
    tags: Mapping[str, str],
    releases: Mapping[str, ReleaseState] | set[str],
    base_branch: str,
    repository: str,
) -> list[ReleaseAction]:
    seed_version = version_tuple(seed_tag)
    if seed_version is None:
        raise ReleaseError(f"Seed tag {seed_tag!r} is not a stable release tag.")
    if tags.get(seed_tag) != seed_commit:
        raise ReleaseError(
            f"Seed tag {seed_tag} must point at {seed_commit}, not {tags.get(seed_tag)!r}."
        )
    if not target_pull_request.merged or target_pull_request.state != "closed":
        raise ReleaseError(
            f"Pull request #{target_pull_request.number} is not verified as merged."
        )
    if target_pull_request.base_ref != base_branch:
        raise ReleaseError(
            f"Pull request #{target_pull_request.number} targets "
            f"{target_pull_request.base_ref!r}, not {base_branch!r}."
        )
    if target_pull_request.base_repository != repository:
        raise ReleaseError(
            f"Pull request #{target_pull_request.number} targets repository "
            f"{target_pull_request.base_repository!r}, not {repository!r}."
        )

    commit_index = {commit: index for index, commit in enumerate(commits)}
    release_point_by_pull_request: dict[int, tuple[int, str, PullRequest]] = {}
    for commit in commits:
        for pull_request in pull_requests_by_commit.get(commit, ()):
            if (
                pull_request.merged
                and pull_request.state == "closed"
                and pull_request.base_ref == base_branch
                and pull_request.base_repository == repository
            ):
                release_point_by_pull_request[pull_request.number] = (
                    commit_index[commit],
                    commit,
                    pull_request,
                )

    target_point = release_point_by_pull_request.get(target_pull_request.number)
    if target_point is None:
        raise ReleaseError(
            f"Merged pull request #{target_pull_request.number} has no verified "
            f"release point on {base_branch}."
        )
    if target_pull_request.merge_commit_sha != target_point[1]:
        raise ReleaseError(
            f"Pull request #{target_pull_request.number} reports merge commit "
            f"{target_pull_request.merge_commit_sha!r}, but its release point is "
            f"{target_point[1]}."
        )

    ordered_points = sorted(release_point_by_pull_request.values(), key=lambda point: point[0])
    commits_with_release_points: set[str] = set()
    for _, commit, _ in ordered_points:
        if commit in commits_with_release_points:
            raise ReleaseError(
                f"Multiple merged pull requests resolve to release point {commit}."
            )
        commits_with_release_points.add(commit)

    tags_by_commit: dict[str, list[str]] = {}
    for tag, commit in tags.items():
        if version_tuple(tag) is not None and commit in commit_index:
            tags_by_commit.setdefault(commit, []).append(tag)
    for commit, commit_tags in tags_by_commit.items():
        if commit not in commits_with_release_points:
            raise ReleaseError(
                f"Stable tag {commit_tags[0]} points at direct/non-PR main commit {commit}."
            )
        if len(commit_tags) > 1:
            raise ReleaseError(
                f"Multiple stable tags point at release point {commit}: "
                f"{', '.join(sorted(commit_tags))}."
            )

    release_states = (
        {tag: ReleaseState() for tag in releases}
        if isinstance(releases, set)
        else dict(releases)
    )
    for index, commit, pull_request in ordered_points:
        if pull_request.merge_commit_sha != commit:
            raise ReleaseError(
                f"Pull request #{pull_request.number} reports merge commit "
                f"{pull_request.merge_commit_sha!r}, but its release point is {commit}."
            )
    target_index = target_point[0]
    actions: list[ReleaseAction] = []
    previous_tag = seed_tag
    previous_version = seed_version
    for index, commit, pull_request in ordered_points:
        if index > target_index:
            break
        commit_tags = tags_by_commit.get(commit, [])
        if commit_tags:
            next_tag = commit_tags[0]
            next_version = version_tuple(next_tag)
            assert next_version is not None
            if next_version <= previous_version:
                raise ReleaseError(
                    f"Manual stable tag {next_tag} must be greater than {previous_tag}."
                )
            create_tag = False
        else:
            major, minor, patch = previous_version
            next_version = major, minor, patch + 1
            next_tag = f"v{next_version[0]}.{next_version[1]}.{next_version[2]}"
            if next_tag in tags:
                raise ReleaseError(
                    f"Stable tag {next_tag} points at {tags[next_tag]}, not {commit}."
                )
            create_tag = True

        release_state = release_states.get(next_tag)
        if release_state and release_state.prerelease:
            raise ReleaseError(
                f"Stable tag {next_tag} has a prerelease GitHub Release; remove or "
                "promote it before retrying."
            )
        if release_state and not release_state.draft:
            if create_tag:
                raise ReleaseError(
                    f"Published GitHub Release {next_tag} exists without its expected tag."
                )
            previous_tag = next_tag
            previous_version = next_version
            continue
        actions.append(
            ReleaseAction(
                pull_request=pull_request.number,
                commit=commit,
                previous_tag=previous_tag,
                next_tag=next_tag,
                create_tag=create_tag,
                create_release=release_state is None,
                publish_draft=bool(release_state and release_state.draft),
            )
        )
        previous_tag = next_tag
        previous_version = next_version

    return actions


class GitRepository:
    def __init__(self, remote: str = "origin") -> None:
        self.remote = remote

    @staticmethod
    def _run(*arguments: str) -> str:
        return subprocess.check_output(["git", *arguments], text=True).removesuffix("\n")

    def fetch(self, branch: str) -> None:
        subprocess.run(
            ["git", "fetch", "--force", "--tags", self.remote], check=True
        )
        subprocess.run(
            [
                "git",
                "fetch",
                "--force",
                self.remote,
                f"+refs/heads/{branch}:refs/remotes/{self.remote}/{branch}",
            ],
            check=True,
        )

    def resolve_tag(self, tag: str) -> str:
        return self._run("rev-list", "-n", "1", tag)

    def first_parent_commits(self, seed_tag: str, branch: str) -> list[str]:
        remote_branch = f"{self.remote}/{branch}"
        subprocess.run(
            ["git", "merge-base", "--is-ancestor", seed_tag, remote_branch],
            check=True,
        )
        output = self._run(
            "rev-list", "--first-parent", "--reverse", f"{seed_tag}..{remote_branch}"
        )
        return output.splitlines() if output else []

    def is_ancestor(self, ancestor: str, descendant: str) -> bool:
        return (
            subprocess.run(
                ["git", "merge-base", "--is-ancestor", ancestor, descendant],
                check=False,
            ).returncode
            == 0
        )

    def stable_tags(self) -> dict[str, str]:
        output = self._run("tag", "--list")
        result: dict[str, str] = {}
        tags = output.removesuffix("\n").split("\n") if output else []
        for tag in tags:
            if TAG_PATTERN.fullmatch(tag):
                result[tag] = self.resolve_tag(tag)
        return result

    def create_tag(self, tag: str, commit: str) -> None:
        existing_tags = self.stable_tags()
        if tag in existing_tags:
            if existing_tags[tag] == commit:
                return
            raise ReleaseError(
                f"Refusing to create {tag}: it now points at {existing_tags[tag]}, "
                f"not {commit}."
            )
        subprocess.run(
            ["git", "tag", "--annotate", tag, commit, "--message", f"Release {tag}"],
            check=True,
        )
        subprocess.run(
            ["git", "push", self.remote, f"refs/tags/{tag}"], check=True
        )


class GitHub:
    def __init__(self, repository: str) -> None:
        self.repository = repository

    @staticmethod
    def _run(*arguments: str) -> str:
        return subprocess.check_output(["gh", *arguments], text=True)

    def api(self, path: str) -> object:
        return json.loads(self._run("api", path))

    def api_paginated(self, path: str) -> list[object]:
        pages = json.loads(self._run("api", "--paginate", "--slurp", path))
        result: list[object] = []
        for page in pages:
            if not isinstance(page, list):
                raise ReleaseError(f"Expected a list response from GitHub API {path}.")
            result.extend(page)
        return result

    def workflow_run(self, run_id: int) -> Mapping[str, object]:
        result = self.api(f"repos/{self.repository}/actions/runs/{run_id}")
        if not isinstance(result, dict):
            raise ReleaseError("GitHub returned an invalid workflow-run response.")
        return result

    def pull_request(self, number: int) -> PullRequest:
        result = self.api(f"repos/{self.repository}/pulls/{number}")
        if not isinstance(result, dict):
            raise ReleaseError(f"GitHub returned an invalid response for PR #{number}.")
        base = result.get("base")
        if not isinstance(base, dict) or not isinstance(base.get("ref"), str):
            raise ReleaseError(f"GitHub returned an invalid base for PR #{number}.")
        base_repository = base.get("repo")
        if not isinstance(base_repository, dict) or not isinstance(
            base_repository.get("full_name"), str
        ):
            raise ReleaseError(
                f"GitHub returned an invalid base repository for PR #{number}."
            )
        head = result.get("head")
        if not isinstance(head, dict):
            raise ReleaseError(f"GitHub returned an invalid head for PR #{number}.")
        head_repository = head.get("repo")
        return PullRequest(
            number=int(result["number"]),
            state=str(result["state"]),
            merged=bool(result.get("merged")),
            base_ref=str(base["ref"]),
            base_repository=str(base_repository["full_name"]),
            merge_commit_sha=(
                str(result["merge_commit_sha"])
                if result.get("merge_commit_sha")
                else None
            ),
            head_ref=str(head["ref"]) if head.get("ref") else None,
            head_sha=str(head["sha"]) if head.get("sha") else None,
            head_repository=(
                str(head_repository["full_name"])
                if isinstance(head_repository, dict) and head_repository.get("full_name")
                else None
            ),
            base_sha=str(base["sha"]) if base.get("sha") else None,
        )

    def commit_pull_request_numbers(self, commit: str) -> list[int]:
        pulls = self.api_paginated(
            f"repos/{self.repository}/commits/{commit}/pulls?per_page=100"
        )
        return [int(pull["number"]) for pull in pulls if isinstance(pull, dict)]

    def release_states(self) -> dict[str, ReleaseState]:
        releases = self.api_paginated(
            f"repos/{self.repository}/releases?per_page=100"
        )
        return {
            str(release["tag_name"]): ReleaseState(
                draft=bool(release.get("draft")),
                prerelease=bool(release.get("prerelease")),
            )
            for release in releases
            if isinstance(release, dict) and release.get("tag_name")
        }

    def release_exists(self, tag: str) -> bool:
        result = subprocess.run(
            ["gh", "release", "view", tag, "--repo", self.repository],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        return result.returncode == 0

    def create_release(self, tag: str, previous_tag: str) -> None:
        if self.release_exists(tag):
            return
        subprocess.run(
            [
                "gh",
                "release",
                "create",
                tag,
                "--repo",
                self.repository,
                "--verify-tag",
                "--title",
                tag,
                "--generate-notes",
                "--notes-start-tag",
                previous_tag,
            ],
            check=True,
        )

    def publish_draft(self, tag: str) -> None:
        subprocess.run(
            [
                "gh",
                "release",
                "edit",
                tag,
                "--repo",
                self.repository,
                "--draft=false",
            ],
            check=True,
        )


def resolve_workflow_pull_request(
    github: GitHub,
    workflow_run: Mapping[str, object],
    repository: str,
) -> PullRequest:
    display_title = workflow_run.get("display_title")
    match = RUN_NAME_PATTERN.fullmatch(display_title) if isinstance(display_title, str) else None
    if not match:
        raise ReleaseError("Upstream workflow run has a malformed deterministic run name.")
    number = int(match.group(1))
    pull_request = github.pull_request(number)

    associations = workflow_run.get("pull_requests", [])
    if associations is None:
        associations = []
    if not isinstance(associations, list):
        raise ReleaseError("GitHub returned invalid workflow-run PR context.")
    if len(associations) > 1:
        raise ReleaseError("Upstream workflow run has ambiguous PR associations.")
    if associations:
        association = associations[0]
        if not isinstance(association, dict) or int(association.get("number", 0)) != number:
            raise ReleaseError("Workflow-run PR association disagrees with its run name.")
        for side, expected_ref, expected_sha, expected_repository in (
            (
                "head",
                pull_request.head_ref,
                pull_request.head_sha,
                pull_request.head_repository,
            ),
            (
                "base",
                pull_request.base_ref,
                pull_request.base_sha,
                pull_request.base_repository,
            ),
        ):
            metadata = association.get(side)
            if not isinstance(metadata, dict):
                continue
            if metadata.get("ref") and expected_ref and metadata["ref"] != expected_ref:
                raise ReleaseError(f"Workflow-run {side} ref disagrees with PR #{number}.")
            if metadata.get("sha") and expected_sha and metadata["sha"] != expected_sha:
                raise ReleaseError(f"Workflow-run {side} SHA disagrees with PR #{number}.")
            metadata_repository = metadata.get("repo")
            if (
                isinstance(metadata_repository, dict)
                and metadata_repository.get("full_name")
                and expected_repository
                and metadata_repository["full_name"] != expected_repository
            ):
                raise ReleaseError(
                    f"Workflow-run {side} repository disagrees with PR #{number}."
                )

    run_head_branch = workflow_run.get("head_branch")
    run_head_sha = workflow_run.get("head_sha")
    run_head_repository = workflow_run.get("head_repository")
    run_head_repository_name = (
        run_head_repository.get("full_name")
        if isinstance(run_head_repository, dict)
        else None
    )
    head_matches = (
        run_head_branch == pull_request.head_ref
        and run_head_sha == pull_request.head_sha
        and run_head_repository_name == pull_request.head_repository
    )
    base_matches = (
        run_head_branch == pull_request.base_ref
        and run_head_repository_name == repository
    )
    if run_head_branch and not (head_matches or base_matches):
        raise ReleaseError("Workflow-run head metadata cannot be bound to the PR.")
    return pull_request


def reconcile(
    *,
    git: GitRepository,
    github: GitHub,
    workflow_run_id: int,
    repository: str,
    base_branch: str,
    seed_tag: str,
) -> list[ReleaseAction]:
    workflow_run = github.workflow_run(workflow_run_id)
    if workflow_run.get("event") != "pull_request_target":
        raise ReleaseError("Upstream workflow run was not a pull_request_target run.")
    if workflow_run.get("name") != "Observe merged PR":
        raise ReleaseError("Upstream workflow run has an unexpected workflow name.")
    workflow_path = workflow_run.get("path")
    if not isinstance(workflow_path, str) or workflow_path.split("@", 1)[0] != (
        ".github/workflows/observe-merged-pr.yml"
    ):
        raise ReleaseError("Upstream workflow run has an unexpected workflow path.")
    run_repository = workflow_run.get("repository")
    if not isinstance(run_repository, dict) or run_repository.get("full_name") != repository:
        raise ReleaseError("Upstream workflow run belongs to an unexpected repository.")
    if workflow_run.get("conclusion") != "success":
        print("Upstream workflow did not succeed; no release reconciliation needed.")
        return []

    target_pull_request = resolve_workflow_pull_request(
        github, workflow_run, repository
    )
    if target_pull_request.state == "open":
        print(f"Pull request #{target_pull_request.number} is still open; nothing to release.")
        return []
    if not target_pull_request.merged:
        print(
            f"Pull request #{target_pull_request.number} closed without merging; "
            "nothing to release."
        )
        return []
    if target_pull_request.base_ref != base_branch:
        raise ReleaseError(
            f"Merged pull request #{target_pull_request.number} targets "
            f"{target_pull_request.base_ref!r}, not {base_branch!r}."
        )
    if target_pull_request.base_repository != repository:
        raise ReleaseError(
            f"Merged pull request #{target_pull_request.number} targets repository "
            f"{target_pull_request.base_repository!r}, not {repository!r}."
        )
    if not target_pull_request.merge_commit_sha:
        raise ReleaseError(
            f"Merged pull request #{target_pull_request.number} has no verifiable "
            "merge commit."
        )

    git.fetch(base_branch)
    seed_commit = git.resolve_tag(seed_tag)
    main_commits = git.first_parent_commits(seed_tag, base_branch)
    positions = {seed_commit: 0}
    positions.update(
        {commit: index + 1 for index, commit in enumerate(main_commits)}
    )
    target_commit = target_pull_request.merge_commit_sha
    if target_commit not in positions:
        raise ReleaseError(
            f"Verified merge commit {target_commit} is not on {base_branch}'s "
            "first-parent ancestry after the release seed."
        )
    run_head_sha = workflow_run.get("head_sha")
    if (
        workflow_run.get("head_branch") == base_branch
        and isinstance(run_head_sha, str)
        and not git.is_ancestor(run_head_sha, f"origin/{base_branch}")
    ):
        raise ReleaseError("Workflow-run base SHA is not reachable from current main.")

    tags = git.stable_tags()
    release_states = github.release_states()
    anchor_tag, anchor_commit = select_completed_anchor(
        seed_tag=seed_tag,
        seed_commit=seed_commit,
        main_commits=main_commits,
        tags=tags,
        releases=release_states,
    )

    pull_request_cache = {target_pull_request.number: target_pull_request}

    def pull_requests_for_commit(commit: str) -> list[PullRequest]:
        result: list[PullRequest] = []
        for number in github.commit_pull_request_numbers(commit):
            if number not in pull_request_cache:
                pull_request_cache[number] = github.pull_request(number)
            result.append(pull_request_cache[number])
        return result

    if anchor_commit != seed_commit:
        anchor_pull_requests = pull_requests_for_commit(anchor_commit)
        if not any(
            pull_request.merged
            and pull_request.base_ref == base_branch
            and pull_request.base_repository == repository
            and pull_request.merge_commit_sha == anchor_commit
            for pull_request in anchor_pull_requests
        ):
            raise ReleaseError(
                f"Completed release anchor {anchor_tag} is not a verified merged-PR point."
            )

    anchor_position = positions[anchor_commit]
    target_position = positions[target_commit]
    if anchor_position >= target_position:
        target_associations = pull_requests_for_commit(target_commit)
        if not any(
            pull_request.number == target_pull_request.number
            and pull_request.merge_commit_sha == target_commit
            for pull_request in target_associations
        ):
            raise ReleaseError("Target PR is not associated with its verified merge commit.")
        print(f"PR #{target_pull_request.number} is at or before completed anchor {anchor_tag}.")
        return []

    commits = main_commits[anchor_position:target_position]
    pull_requests_by_commit: dict[str, list[PullRequest]] = {}
    for commit in commits:
        pull_requests_by_commit[commit] = pull_requests_for_commit(commit)

    actions = plan_releases(
        seed_tag=anchor_tag,
        seed_commit=anchor_commit,
        commits=commits,
        pull_requests_by_commit=pull_requests_by_commit,
        target_pull_request=target_pull_request,
        tags=tags,
        releases=release_states,
        base_branch=base_branch,
        repository=repository,
    )
    for action in actions:
        if action.create_tag:
            git.fetch(base_branch)
            git.create_tag(action.next_tag, action.commit)
        if action.create_release:
            github.create_release(action.next_tag, action.previous_tag)
        elif action.publish_draft:
            github.publish_draft(action.next_tag)
        print(
            f"Reconciled PR #{action.pull_request} at {action.commit} as "
            f"{action.next_tag}."
        )
    if not actions:
        print(
            f"All releases through PR #{target_pull_request.number} are already complete."
        )
    return actions


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Reconcile ordered patch releases for merged pull requests."
    )
    parser.add_argument("--repository", required=True)
    parser.add_argument("--workflow-run-id", required=True, type=int)
    parser.add_argument("--base-branch", default="main")
    parser.add_argument("--seed-tag", default="v0.0.1")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        reconcile(
            git=GitRepository(),
            github=GitHub(args.repository),
            workflow_run_id=args.workflow_run_id,
            repository=args.repository,
            base_branch=args.base_branch,
            seed_tag=args.seed_tag,
        )
    except (ReleaseError, subprocess.CalledProcessError) as error:
        print(f"release reconciliation failed: {error}")
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

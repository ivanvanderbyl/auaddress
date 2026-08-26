#!/usr/bin/env python3

from __future__ import annotations

import sys
import subprocess
import tempfile
import unittest
from contextlib import chdir
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))

from reconcile_releases import (
    GitRepository,
    GitHub,
    PullRequest,
    ReleaseError,
    ReleaseState,
    plan_releases,
    reconcile,
    select_completed_anchor,
)


def merged_pull_request(number: int, commit: str) -> PullRequest:
    return PullRequest(number, "closed", True, "main", "owner/repository", commit)


class PlanReleasesTest(unittest.TestCase):
    seed_tag = "v0.0.1"
    seed_commit = "seed"

    def plan(
        self,
        *,
        commits: list[str],
        pull_requests_by_commit: dict[str, list[PullRequest]],
        target: PullRequest,
        tags: dict[str, str] | None = None,
        releases: set[str] | None = None,
    ) -> list:
        return plan_releases(
            seed_tag=self.seed_tag,
            seed_commit=self.seed_commit,
            commits=commits,
            pull_requests_by_commit=pull_requests_by_commit,
            target_pull_request=target,
            tags=(
                tags
                if tags is not None
                else {self.seed_tag: self.seed_commit}
            ),
            releases=releases if releases is not None else set(),
            base_branch="main",
            repository="owner/repository",
        )

    def test_reversed_event_arrival_releases_earlier_pr_first(self) -> None:
        pull_a = merged_pull_request(10, "commit-a")
        pull_b = merged_pull_request(11, "commit-b")
        commits = ["commit-a", "commit-b"]
        associations = {"commit-a": [pull_a], "commit-b": [pull_b]}

        actions = self.plan(
            commits=commits,
            pull_requests_by_commit=associations,
            target=pull_b,
        )

        self.assertEqual(
            [
                (action.pull_request, action.previous_tag, action.next_tag)
                for action in actions
            ],
            [(10, "v0.0.1", "v0.0.2"), (11, "v0.0.2", "v0.0.3")],
        )

        retry = self.plan(
            commits=commits,
            pull_requests_by_commit=associations,
            target=pull_a,
            tags={
                self.seed_tag: self.seed_commit,
                "v0.0.2": "commit-a",
                "v0.0.3": "commit-b",
            },
            releases={"v0.0.2", "v0.0.3"},
        )
        self.assertEqual(retry, [])

    def test_recovers_when_tag_exists_but_release_is_missing(self) -> None:
        pull_request = merged_pull_request(10, "commit-a")

        actions = self.plan(
            commits=["commit-a"],
            pull_requests_by_commit={"commit-a": [pull_request]},
            target=pull_request,
            tags={self.seed_tag: self.seed_commit, "v0.0.2": "commit-a"},
        )

        self.assertEqual(len(actions), 1)
        self.assertFalse(actions[0].create_tag)
        self.assertTrue(actions[0].create_release)

    def test_already_released_pull_request_is_a_no_op(self) -> None:
        pull_request = merged_pull_request(10, "commit-a")

        actions = self.plan(
            commits=["commit-a"],
            pull_requests_by_commit={"commit-a": [pull_request]},
            target=pull_request,
            tags={self.seed_tag: self.seed_commit, "v0.0.2": "commit-a"},
            releases={"v0.0.2"},
        )

        self.assertEqual(actions, [])

    def test_direct_main_commit_is_not_a_release_point(self) -> None:
        pull_request = merged_pull_request(10, "commit-a")

        actions = self.plan(
            commits=["direct", "commit-a"],
            pull_requests_by_commit={"direct": [], "commit-a": [pull_request]},
            target=pull_request,
        )

        self.assertEqual(len(actions), 1)
        self.assertEqual(actions[0].commit, "commit-a")
        self.assertEqual(actions[0].next_tag, "v0.0.2")

    def test_conflicting_tag_fails(self) -> None:
        pull_request = merged_pull_request(10, "commit-a")

        with self.assertRaisesRegex(ReleaseError, "not commit-a"):
            self.plan(
                commits=["commit-a"],
                pull_requests_by_commit={"commit-a": [pull_request]},
                target=pull_request,
                tags={self.seed_tag: self.seed_commit, "v0.0.2": "wrong"},
            )

    def test_multiple_stable_tags_on_release_history_fail(self) -> None:
        pull_request = merged_pull_request(10, "commit-a")

        with self.assertRaisesRegex(ReleaseError, "must be greater"):
            self.plan(
                commits=["commit-a"],
                pull_requests_by_commit={"commit-a": [pull_request]},
                target=pull_request,
                tags={self.seed_tag: self.seed_commit, "v0.0.0": "commit-a"},
            )

    def test_unverified_earlier_release_point_fails(self) -> None:
        pull_a = merged_pull_request(10, "reported-a")
        pull_b = merged_pull_request(11, "commit-b")

        with self.assertRaisesRegex(ReleaseError, "reports merge commit"):
            self.plan(
                commits=["commit-a", "commit-b"],
                pull_requests_by_commit={
                    "commit-a": [pull_a],
                    "commit-b": [pull_b],
                },
                target=pull_b,
            )

    def test_multiple_pull_requests_at_one_release_point_fail(self) -> None:
        pull_a = merged_pull_request(10, "commit-a")
        pull_b = merged_pull_request(11, "commit-a")

        with self.assertRaisesRegex(ReleaseError, "Multiple merged pull requests"):
            self.plan(
                commits=["commit-a"],
                pull_requests_by_commit={"commit-a": [pull_a, pull_b]},
                target=pull_b,
            )

    def test_merge_commit_is_release_point(self) -> None:
        pull_request = merged_pull_request(10, "merge-commit")

        actions = self.plan(
            commits=["merge-commit"],
            pull_requests_by_commit={"merge-commit": [pull_request]},
            target=pull_request,
        )

        self.assertEqual(actions[0].commit, "merge-commit")

    def test_squash_commit_is_release_point(self) -> None:
        pull_request = merged_pull_request(10, "squash-commit")

        actions = self.plan(
            commits=["squash-commit"],
            pull_requests_by_commit={"squash-commit": [pull_request]},
            target=pull_request,
        )

        self.assertEqual(actions[0].commit, "squash-commit")

    def test_last_rebased_commit_is_release_point(self) -> None:
        pull_request = merged_pull_request(10, "rebased-2")

        actions = self.plan(
            commits=["rebased-1", "rebased-2"],
            pull_requests_by_commit={
                "rebased-1": [pull_request],
                "rebased-2": [pull_request],
            },
            target=pull_request,
        )

        self.assertEqual(len(actions), 1)
        self.assertEqual(actions[0].commit, "rebased-2")

    def test_manual_recovery_tag_becomes_anchor_for_next_patch(self) -> None:
        pull_a = merged_pull_request(10, "commit-a")
        pull_b = merged_pull_request(11, "commit-b")

        actions = self.plan(
            commits=["commit-a", "commit-b"],
            pull_requests_by_commit={"commit-a": [pull_a], "commit-b": [pull_b]},
            target=pull_b,
            tags={self.seed_tag: self.seed_commit, "v1.2.3": "commit-a"},
        )

        self.assertEqual(
            [(a.next_tag, a.create_tag, a.previous_tag) for a in actions],
            [("v1.2.3", False, "v0.0.1"), ("v1.2.4", True, "v1.2.3")],
        )

    def test_draft_release_is_published_during_recovery(self) -> None:
        pull_request = merged_pull_request(10, "commit-a")

        actions = self.plan(
            commits=["commit-a"],
            pull_requests_by_commit={"commit-a": [pull_request]},
            target=pull_request,
            tags={self.seed_tag: self.seed_commit, "v0.0.2": "commit-a"},
            releases={"v0.0.2": ReleaseState(draft=True)},
        )

        self.assertTrue(actions[0].publish_draft)
        self.assertFalse(actions[0].create_release)

    def test_prerelease_for_stable_tag_is_a_conflict(self) -> None:
        pull_request = merged_pull_request(10, "commit-a")
        with self.assertRaisesRegex(ReleaseError, "prerelease"):
            self.plan(
                commits=["commit-a"],
                pull_requests_by_commit={"commit-a": [pull_request]},
                target=pull_request,
                tags={self.seed_tag: self.seed_commit, "v0.0.2": "commit-a"},
                releases={"v0.0.2": ReleaseState(prerelease=True)},
            )


class ReconcileTest(unittest.TestCase):
    def setUp(self) -> None:
        self.git = mock.Mock()
        self.github = mock.Mock()
        self.github.workflow_run.return_value = {
            "event": "pull_request_target",
            "name": "Release PR #10",
            "workflow_id": 987,
            "path": ".github/workflows/observe-merged-pr.yml@main",
            "display_title": "Release PR #10",
            "conclusion": "success",
            "repository": {"full_name": "owner/repository"},
            "pull_requests": [],
            "head_branch": "main",
            "head_sha": "base",
            "head_repository": {"full_name": "owner/repository"},
        }
        self.github.workflow.return_value = {
            "id": 987,
            "name": "Observe merged PR",
            "path": ".github/workflows/observe-merged-pr.yml",
            "state": "active",
        }

    def reconcile(self):
        return reconcile(
            git=self.git,
            github=self.github,
            workflow_run_id=123,
            repository="owner/repository",
            base_branch="main",
            seed_tag="v0.0.1",
        )

    def test_open_pull_request_is_a_read_only_no_op(self) -> None:
        self.github.pull_request.return_value = PullRequest(
            10, "open", False, "main", "owner/repository", None
        )

        self.assertEqual(self.reconcile(), [])
        self.git.fetch.assert_not_called()
        self.git.create_tag.assert_not_called()
        self.github.create_release.assert_not_called()

    def test_empty_associations_use_fork_safe_run_name(self) -> None:
        self.github.pull_request.return_value = PullRequest(
            10, "closed", False, "main", "owner/repository", None
        )
        self.assertEqual(self.reconcile(), [])

    def test_missing_association_field_uses_fork_safe_run_name(self) -> None:
        del self.github.workflow_run.return_value["pull_requests"]
        self.github.pull_request.return_value = PullRequest(
            10, "closed", False, "main", "owner/repository", None
        )
        self.assertEqual(self.reconcile(), [])

    def test_association_number_must_agree_with_run_name(self) -> None:
        self.github.workflow_run.return_value["pull_requests"] = [{"number": 11}]
        self.github.pull_request.return_value = PullRequest(
            10, "closed", False, "main", "owner/repository", None
        )
        with self.assertRaisesRegex(ReleaseError, "disagrees"):
            self.reconcile()

    def test_association_number_may_agree_with_run_name(self) -> None:
        self.github.workflow_run.return_value["pull_requests"] = [{"number": 10}]
        self.github.pull_request.return_value = PullRequest(
            10, "closed", False, "main", "owner/repository", None
        )
        self.assertEqual(self.reconcile(), [])

    def test_association_head_metadata_must_agree(self) -> None:
        self.github.workflow_run.return_value["pull_requests"] = [
            {"number": 10, "head": {"ref": "wrong", "sha": "head"}}
        ]
        self.github.pull_request.return_value = PullRequest(
            10,
            "closed",
            False,
            "main",
            "owner/repository",
            None,
            head_ref="feature",
            head_sha="head",
        )
        with self.assertRaisesRegex(ReleaseError, "head ref disagrees"):
            self.reconcile()

    def test_malformed_run_name_fails(self) -> None:
        self.github.workflow_run.return_value["display_title"] = "Release something"
        with self.assertRaisesRegex(ReleaseError, "malformed"):
            self.reconcile()

    def test_wrong_workflow_identity_fails(self) -> None:
        self.github.workflow_run.return_value["path"] = ".github/workflows/other.yml@main"
        with self.assertRaisesRegex(ReleaseError, "workflow path"):
            self.reconcile()

    def test_wrong_workflow_name_fails(self) -> None:
        self.github.workflow.return_value["name"] = "Preview release tag"
        with self.assertRaisesRegex(ReleaseError, "workflow name"):
            self.reconcile()

    def test_closed_unmerged_pull_request_is_a_read_only_no_op(self) -> None:
        self.github.pull_request.return_value = PullRequest(
            10, "closed", False, "main", "owner/repository", "commit-a"
        )

        self.assertEqual(self.reconcile(), [])
        self.git.fetch.assert_not_called()
        self.git.create_tag.assert_not_called()
        self.github.create_release.assert_not_called()

    def test_verified_merged_pull_request_uses_mocked_git_and_github_writes(self) -> None:
        pull_request = merged_pull_request(10, "commit-a")
        self.github.pull_request.return_value = pull_request
        self.github.commit_pull_request_numbers.return_value = [10]
        self.github.release_states.return_value = {}
        self.git.resolve_tag.return_value = "seed"
        self.git.first_parent_commits.return_value = ["commit-a"]
        self.git.stable_tags.return_value = {"v0.0.1": "seed"}
        self.git.is_ancestor.return_value = True

        actions = self.reconcile()

        self.assertEqual(len(actions), 1)
        self.git.create_tag.assert_called_once_with("v0.0.2", "commit-a")
        self.github.create_release.assert_called_once_with("v0.0.2", "v0.0.1")

    def test_late_target_queries_only_anchor_and_missing_interval(self) -> None:
        pull_b = merged_pull_request(11, "commit-b")
        pull_c = merged_pull_request(12, "commit-c")
        self.github.workflow_run.return_value["display_title"] = "Release PR #12"
        self.github.pull_request.side_effect = lambda number: {
            11: pull_b,
            12: pull_c,
        }[number]
        self.github.commit_pull_request_numbers.side_effect = lambda commit: {
            "commit-b": [11],
            "commit-c": [12],
        }[commit]
        self.github.release_states.return_value = {
            "v0.0.2": ReleaseState(),
            "v0.0.3": ReleaseState(),
        }
        self.git.resolve_tag.return_value = "seed"
        self.git.first_parent_commits.return_value = [
            "commit-a",
            "commit-b",
            "commit-c",
        ]
        self.git.stable_tags.return_value = {
            "v0.0.1": "seed",
            "v0.0.2": "commit-a",
            "v0.0.3": "commit-b",
        }
        self.git.is_ancestor.return_value = True

        actions = self.reconcile()

        self.assertEqual([action.next_tag for action in actions], ["v0.0.4"])
        queried = [call.args[0] for call in self.github.commit_pull_request_numbers.call_args_list]
        self.assertEqual(queried, ["commit-b", "commit-c"])


class CompletedAnchorTest(unittest.TestCase):
    def test_latest_contiguous_published_release_is_anchor(self) -> None:
        anchor = select_completed_anchor(
            seed_tag="v0.0.1",
            seed_commit="seed",
            main_commits=["commit-a", "commit-b", "commit-c"],
            tags={
                "v0.0.1": "seed",
                "v0.0.2": "commit-a",
                "v1.2.3": "commit-b",
            },
            releases={"v0.0.2": ReleaseState(), "v1.2.3": ReleaseState()},
        )
        self.assertEqual(anchor, ("v1.2.3", "commit-b"))

    def test_published_release_after_draft_gap_fails(self) -> None:
        with self.assertRaisesRegex(ReleaseError, "incomplete earlier"):
            select_completed_anchor(
                seed_tag="v0.0.1",
                seed_commit="seed",
                main_commits=["commit-a", "commit-b"],
                tags={
                    "v0.0.1": "seed",
                    "v0.0.2": "commit-a",
                    "v0.0.3": "commit-b",
                },
                releases={
                    "v0.0.2": ReleaseState(draft=True),
                    "v0.0.3": ReleaseState(),
                },
            )


class GitRepositoryTest(unittest.TestCase):
    def test_stable_tags_do_not_normalize_unicode_whitespace(self) -> None:
        repository = GitRepository()

        with (
            mock.patch.object(
                repository,
                "_run",
                return_value="v1.2.3\nv9.9.9\N{NO-BREAK SPACE}",
            ),
            mock.patch.object(repository, "resolve_tag", return_value="commit"),
        ):
            tags = repository.stable_tags()

        self.assertEqual(tags, {"v1.2.3": "commit"})

    def test_stable_tags_do_not_split_on_unicode_line_separator(self) -> None:
        repository = GitRepository()

        with (
            mock.patch.object(
                repository,
                "_run",
                return_value="v1.2.3\nv9.9.9\N{LINE SEPARATOR}",
            ),
            mock.patch.object(repository, "resolve_tag", return_value="commit"),
        ):
            tags = repository.stable_tags()

        self.assertEqual(tags, {"v1.2.3": "commit"})

    def test_temporary_repository_first_parent_tags_and_recovery(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            repository_path = Path(directory)
            subprocess.run(["git", "init", "-b", "main"], cwd=directory, check=True)
            subprocess.run(
                ["git", "config", "user.name", "Test"], cwd=directory, check=True
            )
            subprocess.run(
                ["git", "config", "user.email", "test@example.com"],
                cwd=directory,
                check=True,
            )
            tracked = repository_path / "tracked"
            tracked.write_text("seed\n", encoding="utf-8")
            subprocess.run(["git", "add", "tracked"], cwd=directory, check=True)
            subprocess.run(["git", "commit", "-m", "seed"], cwd=directory, check=True)
            subprocess.run(
                ["git", "tag", "-a", "v0.0.1", "-m", "seed"],
                cwd=directory,
                check=True,
            )
            tracked.write_text("release\n", encoding="utf-8")
            subprocess.run(["git", "commit", "-am", "release"], cwd=directory, check=True)
            release_commit = subprocess.check_output(
                ["git", "rev-parse", "HEAD"], cwd=directory, text=True
            ).strip()
            subprocess.run(
                ["git", "tag", "-a", "v0.0.2", "-m", "release"],
                cwd=directory,
                check=True,
            )
            subprocess.run(
                ["git", "tag", "v9.9.9\N{LINE SEPARATOR}"],
                cwd=directory,
                check=True,
            )
            subprocess.run(
                ["git", "update-ref", "refs/remotes/origin/main", "HEAD"],
                cwd=directory,
                check=True,
            )

            with chdir(directory):
                git_repository = GitRepository()
                commits = git_repository.first_parent_commits("v0.0.1", "main")
                tags = git_repository.stable_tags()

            self.assertEqual(commits, [release_commit])
            self.assertEqual(tags["v0.0.2"], release_commit)
            self.assertNotIn("v9.9.9", tags)
            pull_request = merged_pull_request(10, release_commit)
            actions = plan_releases(
                seed_tag="v0.0.1",
                seed_commit=tags["v0.0.1"],
                commits=commits,
                pull_requests_by_commit={release_commit: [pull_request]},
                target_pull_request=pull_request,
                tags=tags,
                releases={},
                base_branch="main",
                repository="owner/repository",
            )
            self.assertFalse(actions[0].create_tag)
            self.assertTrue(actions[0].create_release)


class GitHubClientTest(unittest.TestCase):
    def test_workflow_reads_the_workflow_identity_endpoint(self) -> None:
        github = GitHub("owner/repository")
        with mock.patch.object(
            github,
            "api",
            return_value={"id": 987, "name": "Observe merged PR"},
        ) as api:
            self.assertEqual(
                github.workflow(987),
                {"id": 987, "name": "Observe merged PR"},
            )

        api.assert_called_once_with("repos/owner/repository/actions/workflows/987")

    def test_paginated_api_flattens_pages(self) -> None:
        github = GitHub("owner/repository")
        with mock.patch.object(github, "_run", return_value='[[{"id": 1}], [{"id": 2}]]'):
            self.assertEqual(github.api_paginated("path"), [{"id": 1}, {"id": 2}])

    def test_api_subprocess_failure_propagates(self) -> None:
        github = GitHub("owner/repository")
        error = subprocess.CalledProcessError(1, ["gh", "api"])
        with mock.patch.object(github, "_run", side_effect=error):
            with self.assertRaises(subprocess.CalledProcessError):
                github.api("path")

if __name__ == "__main__":
    unittest.main()

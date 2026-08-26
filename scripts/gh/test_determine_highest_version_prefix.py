#!/usr/bin/env python3

from __future__ import annotations

import io
import os
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))

import determine_highest_version_prefix as version_prefix


class NextVersionTest(unittest.TestCase):
    def test_increments_highest_stable_version_regardless_of_order(self) -> None:
        tags = ["v1.4.8", "v2.0.0", "v1.10.3", "v0.99.99"]

        self.assertEqual(version_prefix.next_version(tags), (2, 0, 1))

    def test_ignores_prerelease_path_and_malformed_tags(self) -> None:
        tags = [
            "v1.2.3-rc.1",
            "cmd/tool/v9.9.9",
            "module/v8.8.8",
            "v01.2.3",
            "v١.٢.٣",
            "v1.2.2",
        ]

        self.assertEqual(version_prefix.next_version(tags), (1, 2, 3))

    def test_ignores_tag_with_unicode_trailing_whitespace(self) -> None:
        tags = ["v1.2.3", "v9.9.9\N{NO-BREAK SPACE}"]

        self.assertEqual(version_prefix.next_version(tags), (1, 2, 4))

    def test_uses_fallback_when_no_stable_version_exists(self) -> None:
        tags = ["v1.2.3-beta.1", "module/v3.2.1", "not-a-version"]

        self.assertEqual(version_prefix.next_version(tags), (0, 1, 0))

    def test_compares_large_components_numerically(self) -> None:
        tags = ["v10.2.3", "v9.100.100", "v10.11.9", "v10.11.10"]

        self.assertEqual(version_prefix.next_version(tags), (10, 11, 11))


class GitTagsTest(unittest.TestCase):
    def test_splits_only_newline_delimited_git_output(self) -> None:
        output = "v1.2.3\nv9.9.9\N{NO-BREAK SPACE}\n"

        with mock.patch.object(subprocess, "check_output", return_value=output):
            self.assertEqual(
                list(version_prefix.git_tags()),
                ["v1.2.3", "v9.9.9\N{NO-BREAK SPACE}"],
            )

    def test_propagates_git_failure(self) -> None:
        error = subprocess.CalledProcessError(1, ["git", "tag"])

        with mock.patch.object(subprocess, "check_output", side_effect=error):
            with self.assertRaises(subprocess.CalledProcessError):
                version_prefix.git_tags()


class MainTest(unittest.TestCase):
    def test_prints_version_and_writes_github_output(self) -> None:
        stdout = io.StringIO()

        with tempfile.TemporaryDirectory() as temporary_directory:
            output_path = Path(temporary_directory) / "github-output"
            output_path.write_text("existing=value\n", encoding="utf-8")
            with (
                mock.patch.object(version_prefix, "git_tags", return_value=["v2.3.4"]),
                mock.patch.dict(os.environ, {"GITHUB_OUTPUT": str(output_path)}),
                redirect_stdout(stdout),
            ):
                result = version_prefix.main()

            self.assertEqual(
                output_path.read_text(encoding="utf-8"),
                "existing=value\nversion=2.3.5\n",
            )

        self.assertEqual(result, 0)
        self.assertEqual(stdout.getvalue(), "2.3.5\n")


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3

from __future__ import annotations

import os
import re
import subprocess
from collections.abc import Iterable


TAG_PATTERN = re.compile(
    r"^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$"
)
FALLBACK_VERSION = (0, 1, 0)


def git_tags() -> Iterable[str]:
    output = subprocess.check_output(["git", "tag"], text=True)
    return output.split()


def next_version(tags: Iterable[str]) -> tuple[int, int, int]:
    versions = []
    for tag in tags:
        match = TAG_PATTERN.match(tag.strip())
        if match:
            versions.append(tuple(map(int, match.groups())))

    if not versions:
        return FALLBACK_VERSION

    major, minor, patch = max(versions)
    return major, minor, patch + 1


def write_output(version: str) -> None:
    output_path = os.environ.get("GITHUB_OUTPUT")
    if not output_path:
        return

    with open(output_path, "a", encoding="utf-8") as output_file:
        output_file.write(f"version={version}\n")


def main() -> int:
    major, minor, patch = next_version(git_tags())
    version = f"{major}.{minor}.{patch}"
    write_output(version)
    print(version)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

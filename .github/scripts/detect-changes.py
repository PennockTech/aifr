#!/usr/bin/env python3
"""Detect which CI job categories need to run based on changed files.

For pull requests, compares against the PR base SHA.
For pushes to main, finds the last successful CI run that is an ancestor of
the current commit, ensuring force-pushes don't mask broken state.

Outputs GitHub Actions outputs: go, goreleaser, markdown, workflows
Each is 'true' or 'false'.
"""

import json
import os
import subprocess
import sys


def run(cmd, *, check=True, capture=True):
    """Run a command, return stdout stripped."""
    r = subprocess.run(
        cmd,
        capture_output=capture,
        text=True,
        check=check,
    )
    return r.stdout.strip() if capture else ""


def gh_api(endpoint):
    """Call the GitHub API via gh CLI, return parsed JSON."""
    result = subprocess.run(
        ["gh", "api", endpoint],
        capture_output=True,
        text=True,
        check=True,
    )
    return json.loads(result.stdout)


def is_ancestor(ancestor, descendant):
    """Check if ancestor is an ancestor of descendant in git history."""
    r = subprocess.run(
        ["git", "merge-base", "--is-ancestor", ancestor, descendant],
        capture_output=True,
    )
    return r.returncode == 0


def find_last_green_ancestor(repo, workflow_file, head_sha):
    """Find the most recent successful CI run whose SHA is an ancestor of HEAD.

    Walks backwards through successful runs (up to 10 pages) to find one
    whose head_sha is a git ancestor of the current commit.  This handles
    force-pushes correctly: if main was reset, the old "green" commit is
    no longer an ancestor so we skip it.
    """
    for page in range(1, 11):
        endpoint = (
            f"repos/{repo}/actions/workflows/{workflow_file}"
            f"/runs?branch=main&status=success&per_page=10&page={page}"
        )
        try:
            data = gh_api(endpoint)
        except subprocess.CalledProcessError:
            print("::warning::Failed to query GitHub API for last successful run", file=sys.stderr)
            return None

        runs = data.get("workflow_runs", [])
        if not runs:
            break

        for run_info in runs:
            candidate = run_info["head_sha"]
            if is_ancestor(candidate, head_sha):
                return candidate

    return None


def categorize_files(changed_files):
    """Categorize changed files into job categories."""
    categories = {
        "go": False,
        "goreleaser": False,
        "markdown": False,
        "workflows": False,
    }

    for f in changed_files:
        if f.endswith(".go") or f in ("go.mod", "go.sum"):
            categories["go"] = True
        if f == "Taskfile.yml":
            # Taskfile affects all build/test commands
            categories["go"] = True
            categories["goreleaser"] = True
        if f == "goreleaser.yaml":
            categories["goreleaser"] = True
            # goreleaser builds Go, so test Go too
            categories["go"] = True
        if f.endswith(".md"):
            categories["markdown"] = True
        if f.startswith(".github/"):
            categories["workflows"] = True

    return categories


def get_changed_files(base_sha, head_sha):
    """Get list of files changed between two commits."""
    try:
        output = run(["git", "diff", "--name-only", f"{base_sha}...{head_sha}"])
        return [f for f in output.splitlines() if f]
    except subprocess.CalledProcessError:
        # If three-dot diff fails (e.g., shallow clone), try two-dot
        try:
            output = run(["git", "diff", "--name-only", f"{base_sha}..{head_sha}"])
            return [f for f in output.splitlines() if f]
        except subprocess.CalledProcessError:
            return None


def write_outputs(categories):
    """Write category flags to GITHUB_OUTPUT."""
    output_file = os.environ.get("GITHUB_OUTPUT")
    if output_file:
        with open(output_file, "a") as f:
            for key, value in categories.items():
                f.write(f"{key}={'true' if value else 'false'}\n")
    # Always print to stderr for logging
    for key, value in categories.items():
        print(f"  {key}: {value}", file=sys.stderr)


def main():
    event_name = os.environ.get("GITHUB_EVENT_NAME", "")
    repo = os.environ.get("GITHUB_REPOSITORY", "")
    head_sha = os.environ.get("GITHUB_SHA", "")

    # Workflow filename for API queries (must match the actual file)
    workflow_file = "ci.yaml"

    print(f"Event: {event_name}", file=sys.stderr)
    print(f"Repository: {repo}", file=sys.stderr)
    print(f"HEAD SHA: {head_sha}", file=sys.stderr)

    base_sha = None

    if event_name == "pull_request":
        # For PRs, use the base branch SHA
        event_path = os.environ.get("GITHUB_EVENT_PATH", "")
        if event_path:
            with open(event_path) as f:
                event_data = json.load(f)
            base_sha = event_data.get("pull_request", {}).get("base", {}).get("sha")
        print(f"PR base SHA: {base_sha}", file=sys.stderr)

    elif event_name == "push":
        # For pushes to main, find the last green ancestor
        print("Finding last successful CI run that is an ancestor...", file=sys.stderr)
        base_sha = find_last_green_ancestor(repo, workflow_file, head_sha)
        if base_sha:
            print(f"Last green ancestor: {base_sha}", file=sys.stderr)
        else:
            print("No green ancestor found", file=sys.stderr)

    # workflow_dispatch or fallback: no base, run everything
    if not base_sha:
        print("No base SHA available -- running all checks", file=sys.stderr)
        categories = {k: True for k in ("go", "goreleaser", "markdown", "workflows")}
        write_outputs(categories)
        return

    changed_files = get_changed_files(base_sha, head_sha)
    if changed_files is None:
        print("::warning::git diff failed -- running all checks", file=sys.stderr)
        categories = {k: True for k in ("go", "goreleaser", "markdown", "workflows")}
        write_outputs(categories)
        return

    if not changed_files:
        print("No files changed -- skipping all checks", file=sys.stderr)
        categories = {k: False for k in ("go", "goreleaser", "markdown", "workflows")}
        write_outputs(categories)
        return

    print(f"Changed files ({len(changed_files)}):", file=sys.stderr)
    for f in changed_files:
        print(f"  {f}", file=sys.stderr)

    categories = categorize_files(changed_files)
    write_outputs(categories)


if __name__ == "__main__":
    main()

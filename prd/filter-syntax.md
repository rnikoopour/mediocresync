# PRD: Sync Job Filter System

## Overview

Each sync job supports four independent filter lists that control which remote files are included in a transfer. Filters are configured per-job and evaluated at runtime against every file discovered during the remote directory walk.

---

## Filter Types

There are four filter lists:

| Field | Type | Purpose |
|---|---|---|
| `include_path_filters` | `[]string` | Allowlist of subdirectories to descend into |
| `include_name_filters` | `[]string` | Allowlist of filename glob patterns |
| `exclude_path_filters` | `[]string` | Blocklist of subdirectories to skip |
| `exclude_name_filters` | `[]string` | Blocklist of filename glob patterns |

All four default to empty (no filtering).

---

## Evaluation Logic

Filters are applied in this order:

1. **Include path check** — if `include_path_filters` is non-empty, the file must be under at least one listed subdirectory. If it is not, the file is excluded.
2. **Include name check** — if `include_name_filters` is non-empty, the file's basename must match at least one glob pattern. If it does not, the file is excluded.
3. **Exclude path check** — if the file is under any subdirectory in `exclude_path_filters`, the file is excluded.
4. **Exclude name check** — if the file's basename matches any pattern in `exclude_name_filters`, the file is excluded.
5. If all checks pass, the file is included.

Key semantics:
- **Within each include group**: entries are ORed (matching any one is sufficient)
- **Between include groups** (path vs. name): ANDed (both must match if both are non-empty)
- **Exclude groups**: ORed and take final precedence — a matching exclude always wins
- **Empty list = no constraint**: an empty filter list for any type means that type's check is skipped entirely

---

## Path Filter Matching

Path filter entries are subdirectory names or paths relative to the job's remote root. Leading and trailing slashes on entries are stripped before matching, so `alpha` and `/alpha` are equivalent.

### Literal Entries (no wildcards)

A literal entry matches when the file's absolute remote path is exactly equal to — or is nested anywhere under — the resolved subdirectory. Given a job remote path of `/foo/bar` and a path filter entry of `alpha`:

- `/foo/bar/alpha/item.dat` — **matches** (direct child)
- `/foo/bar/alpha/deep/nested/item.dat` — **matches** (nested arbitrarily deep)
- `/foo/bar/alpha2/item.dat` — **does not match** (partial name prefix is rejected)
- `/foo/bar/item.dat` — **does not match** (file is at the remote root, not inside `alpha`)

Entries can contain path separators to target nested subdirectories, e.g. `projects/work` matches files under `/foo/bar/projects/work/`.

### Glob Entries (`**` wildcard)

When an entry contains `*`, `?`, or `[`, it is treated as a glob pattern matched against the file's full path relative to the remote root. The `**` wildcard is supported for cross-directory matching.

| Wildcard | Meaning |
|---|---|
| `*` | Any sequence of characters within a single path segment (does not cross `/`) |
| `**` | Zero or more complete path segments (crosses `/`) |
| `?` | Any single character within a segment |
| `[abc]`, `[a-z]` | Any character in the set or range |

`**` must occupy a complete path segment — i.e., it must be surrounded by `/` or appear at the start or end of the pattern.

**Examples:**

| Entry | Matches | Does not match |
|---|---|---|
| `**/*alpha*` | `sub_alpha_string/file.dat`, `foo/bar/sub_alpha_string/file.dat` | `foo/bar/file.dat` |
| `**/tmp` | `tmp/file.dat`, `foo/bar/tmp/file.dat` | `foo/bar/tmp2/file.dat` |
| `alpha/*/data` | `alpha/jan/data/file.dat`, `alpha/feb/data/file.dat` | `alpha/jan/feb/data/file.dat` |
| `**/[0-9][0-9][0-9][0-9]` | `2024/file.dat`, `archive/2024/file.dat` | `archive/24/file.dat` |

---

## Name Filter Matching (Glob)

Name filters are matched against the **basename only** (the last path component) using Go's `path.Match()`.

Supported glob characters:

| Pattern | Matches |
|---|---|
| `*` | Any sequence of non-`/` characters |
| `?` | Any single non-`/` character |
| `[abc]` | Any character in the set |
| `[a-z]` | Any character in the range |

Notable behaviors:
- `*` does **not** cross directory separators — name filters cannot match path structure
- `.` is a literal character, not a wildcard
- Patterns match the full basename (e.g., `*.txt` matches `report.txt` but not `report.txt.bak`)

---

## Examples

| Scenario | Configuration |
|---|---|
| Only files under `alpha/` | `include_path_filters: ["alpha"]` |
| Only `.dat` files anywhere | `include_name_filters: ["*.dat"]` |
| Only `.dat` files under `alpha/` | `include_path_filters: ["alpha"]`, `include_name_filters: ["*.dat"]` |
| All files except those under `tmp/` | `exclude_path_filters: ["tmp"]` |
| All files except `.cfg` and `.tmp` | `exclude_name_filters: ["*.cfg", "*.tmp"]` |
| Files under `alpha/` or `beta/`, excluding `.log` | `include_path_filters: ["alpha", "beta"]`, `exclude_name_filters: ["*.log"]` |
| Files under any directory whose name contains "sub_alpha_string" | `include_path_filters: ["**/*alpha*"]` |
| Exclude any `tmp` directory at any depth | `exclude_path_filters: ["**/tmp"]` |

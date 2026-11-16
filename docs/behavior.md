# Filesystem behavior

For installation and examples, see the [README](../README.md).

## Selection and normalization

The command accepts exactly one nonempty path:

```text
fpn [options] <path>
```

Without recursion, the selected file, symlink, or directory is a single entry.
Only its basename is normalized; a selected directory's contents are not
processed.

With `--recursive` (`-r`), a selected directory is walked to select files and
symlinks at every depth. No directory is renamed, including the root. Symlinks
encountered during traversal are entries, not traversal targets. A root symlink
is likewise treated as a single link, even if its target is a directory.
Renaming a symlink does not change its stored target.

Normalization trims leading and trailing whitespace, collapses whitespace runs
to a single space, replaces spaces and underscores with hyphens, lowercases the
name, and collapses repeated hyphens. For example:

| Original basename    | Normalized basename |
| -------------------- | ------------------- |
| `My File.txt`        | `my-file.txt`       |
| `Another_File.md`    | `another-file.md`   |
| `Mixed_Case  Foo.MD` | `mixed-case-foo.md` |
| `hello -world`       | `hello-world`       |
| `a  -  b`            | `a-b`               |

Already normalized entries need no rename. `.` and `..` are directory selectors,
not names to normalize. Use the directory's stored basename from its parent to
select it for a nonrecursive rename.

## Path resolution and exact source names

A trailing slash on the argument is ignored. It does not make a final symlink a
traversal target. Parent path components are resolved before simplifying `..`,
so paths through symlinks select the filesystem entry rather than a lexically
simplified substitute. Use `.` explicitly for the current directory; an empty
argument is an error.

Reports consistently use resolved absolute physical paths. Parent symlink
components are resolved, but a final symlink remains the selected directory
entry, not its target. Reported paths therefore need not match the spelling of
the command-line argument.

A supplied source basename must exactly match the stored directory-entry name. A
case-insensitive or Unicode-normalization-insensitive filesystem may accept
other spellings for lookup, but `fpn` rejects those source aliases instead of
silently choosing a stored spelling. Unsupported or ambiguous aliases, such as
Windows short-name aliases, are rejected rather than guessed. Parent directories
must be readable for stored-name and alias checks.

## Exclusions

`.git` and `.DS_Store` are always excluded, including case variants. Excluded
directories are not traversed. Explicitly selecting an excluded entry or a
descendant does not bypass the exclusion; paths reaching excluded locations
through parent symlink aliases are excluded too. Filesystem aliases of excluded
names do not bypass these checks.

These exclusions are independent of the recursive directory-preservation rule.
Other directories, such as `node_modules`, are traversed during recursion, but
their names remain unchanged.

## Whole-plan validation

Traversal and planning finish before any rename starts. Any traversal error,
unreadable parent, invalid source, or collision aborts the whole operation with
a nonzero exit status. There is no partial safe subset: unrelated valid renames
are not applied when the plan contains an error.

A destination occupied by a different directory entry is a collision, including
a hard link to the same file or a symlink, even a dangling symlink. Multiple
sources normalizing to the same destination are also a collision.

A destination that aliases its own source under the filesystem's lookup rules is
rejected too. In particular, changing `Foo.txt` to `foo.txt` on a
case-insensitive filesystem is not supported. On a case-sensitive filesystem,
the same rename can proceed if `foo.txt` is unoccupied. There is no
temporary-name mechanism for case-only or other alias renames.

`--dry-run` (`-n`) performs the same complete traversal and plan validation
without renaming anything. It reports proposed changes only for a valid plan and
returns a nonzero status for validation errors. It does not test whether
permissions, platform support, or filesystem state will allow a later real
rename.

## Execution and no-overwrite guarantees

Every rename atomically refuses to replace an existing destination, including
one created after planning:

- Linux uses `renameat2(RENAME_NOREPLACE)`.
- macOS uses `renamex_np(RENAME_EXCL)`.
- Windows uses `MoveFileEx` without replacement.

Other platforms, or kernels and filesystems lacking the required operation, fail
safely rather than falling back to an overwriting rename.

A runtime failure stops execution and produces a nonzero exit status. Earlier
completed renames are not rolled back; a valid plan is not a transactional
guarantee. There are no temporary rename directories or recovery steps.

Run against a stable tree. Atomic no-replace protects destinations, but does not
pin source entries or parent directories against concurrent replacement. A
successful dry run is not a reservation, and there is no crash-recovery journal.

## Reporting

Successful execution prints an `ORIGINAL` / `NORMALIZED` table of completed
renames; a successful dry run prints the proposed renames. Both columns use
resolved absolute physical paths. Unchanged and excluded entries do not appear
as renames. Execution failures still report earlier completed renames.

Paths containing control characters or invalid UTF-8 are quoted and escaped,
including in diagnostics, so path contents cannot masquerade as extra output
lines. Output failures produce a nonzero exit status and do not roll back
completed renames.

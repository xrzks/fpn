# fpn

A small Go CLI that normalizes filenames: lowercases the base name, replaces spaces and underscores with `-`, leaves parent directories untouched, and skips `.git` and `.DS_Store`.

## Install

```
go install github.com/xrzks/fpn@latest
```

## Usage

```
fpn [--dry] [--recursive] <path>...
```

- `--dry` — print what would happen, without renaming anything. Collisions and other errors still produce a non-zero exit status.
- `--recursive`, `-r` — also rename everything nested inside directory arguments (files and subdirectories, at every depth). The directory argument itself is renamed too. Without this flag, each argument, file or directory, is a single entry and nothing inside a directory is read or touched.

- `--help`, `-h` — show command usage and available flags.

Use `--` before paths beginning with a dash (for example, `fpn -- "-My File.txt"`).

Directories are renamed after their contents, so nested paths are moved along with their (already normalized) parent.

**Breaking change:** the `--dirs` flag has been removed. Directories are now ordinary entries, subject to the same rule as files. Passing `--dirs` is an error.

## Behavior

- Only the final path component of each entry is renamed. Parent directories are left as-is.
- Leading and trailing whitespace is trimmed. The base name is lowercased, runs of whitespace are collapsed to a single space, then every space and underscore becomes `-`. Runs of two or more hyphens are then collapsed to a single `-`. Examples:
  - `My File.txt` → `my-file.txt`
  - `Another_File.md` → `another-file.md`
  - `Mixed_Case  Foo.MD` → `mixed-case-foo.md`
  - `hello -world` → `hello-world`
  - `a  -  b` → `a-b`
- A trailing slash on an argument is ignored (`fpn "Test Dir/"` is the same as `fpn "Test Dir"`). Parent components are resolved before simplifying `..`, so paths through symlinks select the correct entry. Reports may use resolved paths for such arguments.
- Empty path arguments are rejected before any traversal or renames, including when mixed with valid paths. Use `.` explicitly to select the current directory.
- `.` and `..` normalize to themselves, so they are never renamed. With `--recursive` their contents are still walked. To rename the current directory, run `fpn` from its parent.
- `.git` and `.DS_Store` are always skipped, including case variants, explicit descendants, and descendants reached through parent symlink aliases.
- Recursion renames directories too (everything except `.git` and `.DS_Store`, including e.g. `node_modules`). There is no files-only mode, so run with `--dry` first.
- Duplicate paths (the same path given twice, or overlapping arguments such as `-r Dir Dir/Sub`) are processed once.
- If a normalized name already exists on disk as a different file, `fpn` reports a collision and leaves that pair alone — but other safe renames in the same run still proceed and the process exits non-zero so scripts can detect the partial failure.
- Renames atomically refuse to replace a destination created after planning, including during a case-only rename. Linux uses `renameat2(RENAME_NOREPLACE)`, macOS uses `renamex_np(RENAME_EXCL)`, and Windows uses `MoveFileEx` without replacement. Other platforms, or kernels/filesystems without the required operation, fail safely instead of falling back to an overwriting rename. Execution errors stop the run; earlier renames are not rolled back.
- If a case-only rename cannot finish, `fpn` attempts to restore the original name without replacing any new entry there. If restoration also fails, the error reports the temporary path where the source is preserved for manual recovery.
- If several entries would normalize to the same name (e.g. `A b` and `a_b` both become `a-b`), `fpn` reports one collision naming the target and all sources, and renames none of them. This counts toward the non-zero exit.
- Arguments identify stored directory-entry names, even when supplied under different case. Case and Unicode-normalization aliases are checked using the parent filesystem's lookup behavior. A destination alias of the source is renamed via a temporary name; separate entries, including hard links and symlinks (even dangling ones), are collisions. Unsupported or ambiguous source aliases, such as Windows short-name aliases, are rejected rather than guessed.
- Planning reads each parent directory once to identify stored names and aliases. Unreadable parents are errors. A dry run validates the plan, not permissions or the availability of the platform's rename operation.
- Symlinks are renamed as links, without following them or changing their stored targets. Recursive walks do not traverse symlinked directories.
- On success (or partial success), a table of `ORIGINAL` → `NORMALIZED` is printed. Execution failures still report earlier completed renames. Paths containing control characters or invalid UTF-8 are quoted and escaped; output failures produce a non-zero exit status.

Run against a stable tree: atomic no-replace protects destinations, but does not pin source entries or parent directories against concurrent replacement. There is no crash-recovery journal. Interrupting a case-only rename can leave the source at `.fpn-*/entry` in its original parent; inspect that location before removing temporary directories.

## Example

Given this tree:

```
Test Dir/
  Child One/file.txt
  Child Two/
```

```
$ fpn --dry "Test Dir"
ORIGINAL  NORMALIZED
Test Dir  test-dir

$ fpn --dry --recursive "Test Dir"
ORIGINAL            NORMALIZED
Test Dir            test-dir
Test Dir/Child One  Test Dir/child-one
Test Dir/Child Two  Test Dir/child-two

$ fpn --recursive "Test Dir"
```

Result:

```
test-dir/
  child-one/file.txt
  child-two/
```

`Child One/file.txt` is already normalized, so it does not appear in the table. Parent components in the table are the original, on-disk paths; deepest paths are renamed first.

To rename the entries directly inside the current directory, use shell globbing (each matched directory is renamed as a single entry, not expanded):

```
fpn ./*        # add ./.* to include dotfiles
```

To rename an entire tree, use `fpn --recursive .` (`.` itself is not renamed).

## Development

The root `main.go` is a thin entry point, preserving `go run .` and the install command above.

- `internal/cli` defines the command and flags using `urfave/cli/v3`.
- `internal/normalize` owns traversal, normalization planning, reporting, and platform-specific safe renames. It returns errors to the caller and supports injected output writers.

Run the tests with `go test ./...` and build with `go build .`.

## License

MIT. See [LICENSE](./LICENSE).

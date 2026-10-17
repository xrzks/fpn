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

- `--dry` — print what would happen, without renaming anything.
- `--recursive` — also rename everything nested inside directory arguments (files and subdirectories, at every depth). The directory argument itself is renamed too. Without this flag, each argument, file or directory, is a single entry and nothing inside a directory is read or touched.

Directories are renamed after their contents, so nested paths are moved along with their (already normalized) parent.

**Breaking change:** the `--dirs` flag has been removed. Directories are now ordinary entries, subject to the same rule as files. Passing `--dirs` is an error.

## Behavior

- Only the final path component of each entry is renamed. Parent directories are left as-is.
- The base name is lowercased, runs of whitespace are collapsed to a single space, then every space and underscore becomes `-`. Runs of two or more hyphens are then collapsed to a single `-`. Examples:
  - `My File.txt` → `my-file.txt`
  - `Another_File.md` → `another-file.md`
  - `Mixed_Case  Foo.MD` → `mixed-case-foo.md`
  - `hello -world` → `hello-world`
  - `a  -  b` → `a-b`
- A trailing slash on an argument is ignored (`fpn "Test Dir/"` is the same as `fpn "Test Dir"`).
- `.` and `..` normalize to themselves, so they are never renamed. With `--recursive` their contents are still walked. To rename the current directory, run `fpn` from its parent.
- `.git` and `.DS_Store` are always skipped, including everything inside a skipped directory.
- Recursion renames directories too (everything except `.git` and `.DS_Store`, including e.g. `node_modules`). There is no files-only mode, so run with `--dry` first.
- Duplicate paths (the same path given twice, or overlapping arguments such as `-r Dir Dir/Sub`) are processed once.
- If a normalized name already exists on disk as a different file, `fpn` reports a collision and leaves that pair alone — but other safe renames in the same run still proceed and the process exits non-zero so scripts can detect the partial failure.
- If several entries would normalize to the same name (e.g. `A b` and `a_b` both become `a-b`), `fpn` reports one collision naming the target and all sources, and renames none of them. This counts toward the non-zero exit.
- A name that already refers to the same file under different case (e.g. `LICENSE` and `license` on a case-insensitive filesystem like APFS) is not treated as a collision; it is renamed via a temporary name.
- On success (or partial success), a table of `ORIGINAL` → `NORMALIZED` is printed.

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

## License

MIT. See [LICENSE](./LICENSE).

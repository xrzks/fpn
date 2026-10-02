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
- `--recursive` — descend into subdirectories. Without this flag, only files directly inside each given directory are renamed.

## Behavior

- Only the final path component of each argument is renamed. Parent directories are left as-is.
- The base name is lowercased, runs of whitespace are collapsed to a single space, then every space and underscore becomes `-`. Examples:
  - `My File.txt` → `my-file.txt`
  - `Another_File.md` → `another-file.md`
  - `Mixed_Case  Foo.MD` → `mixed-case-foo.md`
- `.git` and `.DS_Store` are always skipped.
- If a normalized name already exists on disk as a different file, `fpn` reports a collision and leaves that pair alone — but other safe renames in the same run still proceed and the process exits non-zero so scripts can detect the partial failure.
- A name that already refers to the same file under different case (e.g. `LICENSE` and `license` on a case-insensitive filesystem like APFS) is not treated as a collision and is skipped silently — there is nothing to rename.
- On success (or partial success), a table of `ORIGINAL` → `NORMALIZED` is printed.

## Example

```
$ ls
My File.txt   Another_File.md   Mixed_Case Foo.MD

$ fpn --dry .
 ORIGINAL             NORMALIZED
 My File.txt          my-file.txt
 Another_File.md      another-file.md
 Mixed_Case Foo.MD    mixed-case-foo.md

$ fpn .
 ORIGINAL             NORMALIZED
 My File.txt          my-file.txt
 Another_File.md      another-file.md
 Mixed_Case Foo.MD    mixed-case-foo.md

$ ls
my-file.txt   another-file.md   mixed-case-foo.md
```

## License

MIT. See [LICENSE](./LICENSE).
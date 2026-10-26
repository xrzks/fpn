# fpn

A small Go CLI that normalizes filenames: lowercases names, replaces whitespace and underscores with `-`, and skips `.git` and `.DS_Store`.

## Install

```sh
go install github.com/xrzks/fpn@latest
```

## Usage

```text
fpn [--dry] [-r] <path>
```

Exactly one path is required.

- Without `-r`, rename one file, symlink, or directory. A directory's contents are not processed.
- `--recursive`, `-r` — for a directory, rename files and symlinks inside it at every depth. Never rename directories, including the selected root. Symlinked directories are not traversed; a symlink supplied as the path is treated as one link.
- `--dry` — validate the entire plan and print proposed renames without changing anything. Any collision or planning error fails the run; no partial safe subset is applied.
- `--help`, `-h` — show usage and flags.

Use `--` before a path beginning with a dash: `fpn -- "-My File.txt"`.

## Behavior

- Only an entry's basename changes; parent directories are untouched. Leading and trailing whitespace is trimmed, whitespace runs become `-`, underscores become `-`, names are lowercased, and repeated hyphens collapse. For example, `Mixed_Case  Foo.MD` becomes `mixed-case-foo.md`.
- `.git` and `.DS_Store` are excluded, including case variants, descendants, and paths through aliases.
- Source basenames must exactly match their stored spelling. Case and Unicode-normalization aliases are rejected. A normalized destination that aliases the source is also rejected, so case-only renames on case-insensitive filesystems are not supported.
- Any traversal error, planning error, or collision aborts before any renames. Destinations are never overwritten. A runtime failure stops execution, but earlier completed renames are not rolled back.
- Reports use resolved absolute physical paths, preserving a final symlink as an entry rather than following it. Unsafe path characters are escaped.
- Run against a stable tree: no-overwrite protection does not protect sources or parent directories from concurrent replacement. A dry run validates the plan, not whether later execution will succeed.

See [detailed filesystem behavior](docs/behavior.md) for path resolution, collision checks, exclusions, and platform guarantees.

## Examples

Rename a single entry, including a directory without processing its contents:

```sh
fpn --dry "My File.txt"
fpn "Test Dir"
```

For a separate recursive example, start with:

```text
Test Dir/
  Child One/My File.txt
  Child Two/Another_File.md
```

Preview and apply:

```sh
fpn --dry -r "Test Dir"
fpn -r "Test Dir"
```

Result (all directory names are preserved):

```text
Test Dir/
  Child One/my-file.txt
  Child Two/another-file.md
```

Use `fpn -r .` to process files and symlinks throughout the current tree while preserving directories. Shell globs that expand to multiple paths are not accepted as a single invocation.

## Development

The root `main.go` is a thin entry point, preserving `go run .` and the install command above.

- `internal/cli` defines the command and flags using `urfave/cli/v3`.
- `internal/normalize` owns traversal, normalization planning, reporting, and platform-specific safe renames. It returns errors to the caller and supports injected output writers.

Run the tests with `go test ./...` and build with `go build .`.

## License

MIT. See [LICENSE](./LICENSE).

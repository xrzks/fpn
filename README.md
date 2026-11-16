# fpn

`fpn` normalizes file and directory names: it lowercases names, replaces
whitespace and underscores with `-`, and skips `.git` and `.DS_Store`.

## Install

```sh
go install github.com/xrzks/fpn@latest
```

Requires Go 1.27 or newer.

## Usage

```text
fpn [options] <path>
```

Exactly one path is required. Without `--recursive`, `fpn` renames that one
file, symlink, or directory; a directory's contents are not processed. Use `--`
before a path beginning with a dash: `fpn -- "-My File.txt"`.

## Options

| Flag          | Alias | Description                                                              |
| ------------- | ----- | ------------------------------------------------------------------------ |
| `--dry-run`   | `-n`  | show what would happen without making changes                            |
| `--recursive` | `-r`  | recursively rename files and symlinks, leaving all directories unchanged |
| `--help`      | `-h`  | show help                                                                |
| `--version`   | `-v`  | print the version                                                        |

`--dry-run` validates the entire plan and prints the proposed renames. Any
collision or planning error fails the run; no partial safe subset is applied.

`--recursive` renames files and symlinks inside a directory at every depth. It
never renames directories, including the selected root. Symlinked directories
are not traversed; a symlink supplied as the path is treated as one link.

## Examples

Rename a single entry, including a directory without processing its contents:

```sh
fpn --dry-run "My File.txt"
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
fpn -n -r "Test Dir"
fpn -r "Test Dir"
```

Result (all directory names are preserved):

```text
Test Dir/
  Child One/my-file.txt
  Child Two/another-file.md
```

Use `fpn -r .` to process files and symlinks throughout the current tree while
preserving directories. Shell globs that expand to multiple paths are not
accepted as a single invocation.

## Behavior

- Only an entry's basename changes; parent directories are untouched. Leading
  and trailing whitespace is trimmed, whitespace runs become `-`, underscores
  become `-`, names are lowercased, and repeated hyphens collapse. For example,
  `Mixed_Case  Foo.MD` becomes `mixed-case-foo.md`.
- `.git` and `.DS_Store` are excluded, including case variants, descendants, and
  paths through aliases.
- Source basenames must exactly match their stored spelling. Case and
  Unicode-normalization aliases are rejected. A normalized destination that
  aliases the source is also rejected, so case-only renames on case-insensitive
  filesystems are not supported.
- Any traversal error, planning error, or collision aborts before any renames.
  Destinations are never overwritten. A runtime failure stops execution, but
  earlier completed renames are not rolled back.
- Reports use resolved absolute physical paths, preserving a final symlink as an
  entry rather than following it. Unsafe path characters are escaped.
- Run against a stable tree: no-overwrite protection does not protect sources or
  parent directories from concurrent replacement. A dry run validates the plan,
  not whether later execution will succeed.

See [detailed filesystem behavior](docs/behavior.md) for path resolution,
collision checks, exclusions, and platform guarantees.

## Development

```sh
go test ./...
go vet ./...
go build .
```

`fpn --version` reports the version Go records at install time. To stamp a
version into a local build, use `go build -ldflags "-X main.version=v1.0.0" .`.

The root `main.go` is a thin entry point. `internal/cli` defines the command and
flags using `urfave/cli/v3`. `internal/normalize` owns traversal, normalization
planning, reporting, and platform-specific safe renames; it returns errors to
the caller and supports injected output writers.

## License

MIT. See [LICENSE](LICENSE).

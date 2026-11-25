# Development

How to build, test, and check vita-decrypt on your own machine. The rules the code must follow are in the
[coding standards](coding-standards.md); this is the how-to.

## Setting up

- **Go 1.26 or later.** CI uses the latest stable release.
- **On Linux, for the desktop app:** the Wayland, X11, EGL, and Vulkan development headers. On Debian or Ubuntu:

  ```sh
  sudo apt-get install libwayland-dev libx11-dev libx11-xcb-dev libxkbcommon-x11-dev libgles2-mesa-dev \
    libegl1-mesa-dev libffi-dev libxcursor-dev libvulkan-dev
  ```

- **On macOS, for the desktop app:** the Xcode command-line tools (it uses cgo).
- **On Windows:** nothing more. The desktop app builds without cgo.

The development tools (golangci-lint, govulncheck, actionlint) don't need installing: they're declared in
`tools/go.mod` and `tools/actionlint/go.mod`, and `go tool` builds the pinned version on first use.

## Running

```sh
go run . keygen --name Test             # the command
go run ./cmd/vita-decrypt-gui           # the desktop app
```

## Checking your work

Run what CI runs before opening a pull request:

```sh
go vet ./...
go tool -modfile=tools/go.mod golangci-lint run ./...   # lint and formatting
go tool -modfile=tools/go.mod golangci-lint fmt         # fix formatting (gofumpt and goimports)
go test -race -covermode=atomic -coverprofile=coverage.out ./...
go run ./scripts/coverage coverage.out                  # every package and the total must be 90% or more
go tool -modfile=tools/go.mod govulncheck ./...
go tool -modfile=tools/actionlint/go.mod actionlint     # after changing a workflow
```

- **Coverage locally is one operating system's view.** CI merges the Linux, Windows, and macOS profiles, so a
  Windows-only file counts as covered by the Windows run. A package that's close to 90% locally may be fine in CI, or
  the reverse if you changed another platform's files.
- **Linting another platform's files:** build golangci-lint once for your own platform, then run that binary with
  `GOOS` set. (`GOOS` with `go tool` would build the tool itself for the other platform.) macOS files need cgo, so
  they can only be linted on a Mac.

  ```sh
  go build -modfile=tools/go.mod -o /tmp/golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint
  GOOS=windows /tmp/golangci-lint run ./...
  ```
- **The desktop app's tests** take about a minute under `-race`, most of it laying out every screen in both themes.

## Fuzzing

Every parser of outside input has a `Fuzz` test, and CI runs each for 10 seconds. To run one for longer:

```sh
go test -run '^$' -fuzz '^FuzzParseBlock$' -fuzztime 5m ./scripts/coverage
```

A failure is saved under the package's `testdata/fuzz/` folder. Commit it with the fix, so the case stays in the seed
corpus.

## Screenshots of the desktop app

```sh
VITA_GUI_SHOTS=/tmp/shots go test -run '^TestScreens$' ./cmd/vita-decrypt-gui
```

Renders every screen and state in both themes to PNG files. It needs a GPU or a software OpenGL. Include before and
after images in a pull request that changes how the app looks; see [design](design.md#changing-the-design).

## Windows icon and version info

The Windows builds carry the app icon (`cmd/vita-decrypt-gui/winres/icon.svg`) and version information, which
Explorer shows under Properties > Details and Task Manager uses for the name. `scripts/winres` writes them as
`rsrc_windows_amd64.syso`, the same bytes MinGW's `windres` would make, so nothing needs installing.

- The desktop app's copy is committed with the version "dev". `scripts/build.sh` writes both programs' copies for
  the release version, then puts the committed one back.
- After changing the SVG, run `scripts/windows-icon.sh` and commit both files it writes. It needs `rsvg-convert`
  and ImageMagick 7 (`librsvg2-bin imagemagick` on Debian or Ubuntu).

## Building release files

```sh
scripts/build.sh 1.2.3
```

Builds static, reproducible command-line binaries for Linux, Windows, and macOS, the Windows desktop app, and the
desktop app for the machine you're on, into `dist/` with a `SHA256SUMS` file. The same source and Go version give
the same checksums. Releases themselves are made by the release workflow from a signed tag; see the
[README](../README.md#releasing).

## Dependencies

Dependabot proposes updates weekly. To update by hand:

```sh
go get -u ./... && go mod tidy
(cd tools && go get -u tool && go mod tidy)
(cd tools/actionlint && go get -u tool && go mod tidy)
```

Adding a new dependency needs the maintainer's approval; see the
[dependency policy](coding-standards.md#6-dependencies-and-github-actions).

## Where things are

See [architecture](architecture.md) for the packages and how they fit together.

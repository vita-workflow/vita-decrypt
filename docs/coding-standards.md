# Coding Standards

Status: **adopted**, 2026-11-23. The code and CI were brought in line with these rules on the branch that adopted them
(see [Rolling this out](#rolling-this-out)).

These standards apply to every file in the repository: Go, shell scripts, workflows, and docs. Where they disagree
with a common Go convention, **these standards win**.

Each rule is either **enforced** (a CI check fails the pull request) or **reviewed** (the reviewer checks it). The
[CI gates](#ci-gates) table lists what's enforced and by what.

---

## 1. Naming

### 1.1 The principle

A name must tell a reader what a thing **is** or **does** without making them read its definition. After that, it
should be as short as possible. Descriptive comes first and brevity second. Neither is optional.

- **Descriptive (DAMN):** whole words, the project's own vocabulary, and enough context to read the name where it's
  used, even when that's 80 lines below where it's declared.
- **Brief (DRY):** don't repeat what the package, the type, the receiver, or the parameter type already says. Repeated
  context is clutter.

This deliberately rejects the common Go style of single-letter receivers, one- and two-letter locals, and clipped
abbreviations (`s.i`, `p`, `res`, `fx`, `cbool`). It also rejects the opposite extreme of enterprise naming
(`KeyPairGenerationServiceFactory`, `IDecryptor`, `strOutputPath`). The target is roughly what you'd write in good
C#, minus C#'s ceremony.

### 1.2 Rules

| # | Rule | Bad | Good |
|---|------|-----|------|
| N1 | **Whole words.** Don't abbreviate unless the abbreviation is in the [glossary](#13-allowed-short-names-and-abbreviations). | `res`, `tmp`, `fg`, `sz`, `ic` | `resources`, `tempPath`, `foreground`, `size`, `icon` |
| N2 | **No one- or two-letter names**, except the allowlist in 1.3. | `s.i`, `s.s`, `p []byte`, `r uintptr` | `scanner.position`, `scanner.source`, `buffer []byte`, `result uintptr` |
| N3 | **Receivers are named after their type, as a word.** Never one letter, never `this` or `self`. Use the same name on every method of a type. | `func (s *pathScanner)` | `func (scanner *pathScanner)` |
| N4 | **Functions and methods that act are verb phrases.** Functions that return a value without side effects are named after the value. | `coff(res)`, `ico(name)`, `fx(v)` | `encodeCOFF(resources)`, `loadIcon(name)`, `fixedToFloat(value)` |
| N5 | **Booleans read as true/false statements**, using `is`, `has`, `can`, `should`, or a past participle. | `done()`, `copied`, `flag` | `isAtEnd()`, `wasCopied`, `isLargeArc` |
| N6 | **Don't stutter.** Leave out anything the package, type, or receiver already says. | `vita.VitaKeyPair`, `openState.openFiles` | `vita.KeyPair`, `openState.files` |
| N7 | **No type or scope tags** (no Hungarian notation, no `I` prefix on interfaces, no `m_`/`_` prefixes). | `strName`, `IOpener`, `pathString` | `name`, `Opener`, `path` |
| N8 | **Put units in the name when the type doesn't carry them.** | `timeout int`, `size int64` | `timeoutSeconds int`, `sizeBytes int64`, or `timeout time.Duration` |
| N9 | **Use one word per concept** across the whole codebase. Use the [domain vocabulary](#14-domain-vocabulary). | `encrypt` in one file, `seal` in another | `seal` everywhere |
| N10 | **Length follows scope.** A loop body three lines long can use `index`; a package-level name needs full context. If a name needs more than four words, the thing it names is probably doing too much. Split it. | `privateKeyFileOutputPathForTheUser` | `privateKeyPath` |
| N11 | **Getters are named after what they return**, with no `Get` prefix, because `Get` adds nothing. Setters use `Set`. | `GetFingerprint()` | `Fingerprint()` |
| N12 | **Don't wrap something just to rename it.** A one-line function that only converts or forwards a value is noise. Call the real thing. | `func dpv(v float32) unit.Dp { return unit.Dp(v) }` | `unit.Dp(value)` at the call site |

### 1.3 Allowed short names and abbreviations

These are the only exceptions to N1 and N2. To add one, open a pull request that changes this list.

| Name | Allowed only for |
|------|------------------|
| `err` | an `error` value |
| `ctx` | a `context.Context` |
| `gtx` | a Gio `layout.Context` (Gio's own name, used throughout its API) |
| `ok` | the second value of a comma-ok expression (map lookup, type assertion, channel receive) |
| `i`, `j` | integer loop indexes, in loops no longer than 10 lines |
| `t`, `b`, `f` | `*testing.T`, `*testing.B`, `*testing.F` |

Initialisms are written in all caps (`ID`, `URL`, `URI`, `JSON`, `COFF`, `HRESULT`, `UTF16`, `RGBA`, `SVG`, `ICO`,
`GUI`, `CLI`, `OS`, `COM`), or all lowercase at the start of an unexported name (`urlPath`). This matches the standard
library and Windows APIs we call, so names that mix ours and theirs stay consistent. `dp` and `sp` are Gio units and
count as words.

### 1.4 Domain vocabulary

| Use | Meaning | Don't use |
|-----|---------|-----------|
| **seal** | encrypt a file to a public key | encrypt, lock |
| **open** | the desktop screen and action that decrypts sealed files | unseal, unlock |
| **decrypt** | the library and command operation behind *open* | open (in `internal/vita`) |
| **key pair** | a private key file plus its public key | keys, keyset |
| **public key** / **recipient** | *public key* in UI text, *recipient* in code that deals with age's `Recipient` | pubkey, pk |
| **private key** / **identity** | *private key* in UI text, *identity* in code that deals with age's `Identity` | privkey, sk, id |
| **fingerprint** | the short, stable hash used to compare public keys | hash, thumbprint |
| **folder** | a directory chosen by the user | dir, directory (in UI text) |

### 1.5 What stays as Go requires

The compiler or `go` tool decides these, so they aren't style choices:

- **Visibility is capitalization.** Exported names are `PascalCase`; unexported names are `camelCase`. There is no
  `SCREAMING_CASE`: constants follow the same two forms.
- **Package names** are one lowercase word with no underscores (`vita`, `desktop`). This is the one place brevity
  wins outright, because the package name prefixes every use: `vita.Decrypt`, not `vitadecryption.Decrypt`.
- **File names** are lowercase. Underscores are only for the suffixes the toolchain reads (`_test`, `_windows`,
  `_darwin`, `_linux`, `_amd64`). A file is named after what it contains (`fingerprint.go`), never `utils.go`,
  `helpers.go`, `common.go`, or `misc.go`.
- **Error variables** start with `Err` (`ErrWrongKey`), and error types end with `Error` (`existsError`).

### 1.6 Test names

- A top-level test is named `Test<Unit>` for a table-driven test, or `Test<Unit>_<Scenario>` for a single scenario:
  `TestDecrypt_RefusesToOverwrite`.
- Subtest names are plain sentences that state the condition and the result: `t.Run("wrong key returns
  ErrWrongKey", …)`.
- Table fields are named the same way as everything else: `input`, `wantOutput`, `wantErr`, not `in`, `out`, `e`.

---

## 2. Don't repeat yourself, and don't over-abstract

- **Rule of three.** Two copies of the same logic are acceptable if extracting them would be awkward. A third copy
  must be extracted. Copies of business rules (file naming, key validation, error wording) are extracted at the
  second.
- **One source of truth.** Rules shared by the command and the desktop app live in `internal/vita` and nowhere else.
  The same goes for user-facing strings that both show, version numbers, and file name rules.
- **No magic values.** A literal that means something (a size limit, a file extension, a permission mode) gets a
  named constant once it is used in more than one place, or if its meaning isn't obvious where it's used.
- **Don't build a wrong abstraction.** Extract a function when the copies change together for the same reason. If
  they only look alike, leave them alone. A helper with a `mode` flag that switches between unrelated behaviors is
  worse than two plain functions.
- **Docs follow the same rule.** A policy is stated in one document and linked from the others. For example, the
  dependency policy belongs in [section 6](#6-dependencies-and-github-actions); the README links to it.

---

## 3. Structure and size

| Limit | Value | Enforced by |
|-------|-------|-------------|
| Function length | 60 lines or 40 statements (closures count toward their enclosing function) | `funlen` |
| Cognitive complexity | 15 per function | `gocognit` |
| Nesting | 4 levels (`if`/`for`/`switch` inside one another) | `nestif` |
| Parameters | 5. Above that, pass a struct. | reviewed |
| File length | 500 lines | reviewed |
| Line length | 120 characters | `lll` |

- **Return early.** Handle errors and edge cases first, then the main path without indentation. No `else` after a
  `return`.
- **No mutable package-level state.** Package-level `var`s are for values that never change after `init`: embedded
  assets, compiled patterns, sentinel errors. Everything else is passed in or held by a struct.
- **Platform code stays behind build tags** in `internal/desktop` (or a `_windows`/`_darwin`/`_linux` file). Every
  platform's file exposes the same functions. Keep the logic (parsing, choosing, formatting) in platform-neutral
  files that can be tested everywhere, and keep the platform files to the system calls themselves.
- **`internal/` by default.** Only `main` packages live outside it.
- **Imports** are grouped as standard library, then third-party, then this module, with a blank line between groups
  (`goimports -local github.com/vita-workflow/vita-decrypt`).
- **Formatting** is `gofumpt` (a stricter `gofmt`) plus `goimports`, both run by golangci-lint. Run
  `go tool -modfile=tools/go.mod golangci-lint fmt` instead of formatting by hand.

---

## 4. Errors

- **Never ignore an error.** If discarding one is correct, write `_ = closer.Close() // reason` with the reason.
  (`errcheck`)
- **Wrap with context** using `%w`: `fmt.Errorf("reading %s: %w", path, err)`. The context says what was being done,
  not that it failed ("reading x", not "failed to read x"), so wrapped messages read cleanly when chained.
- **Compare with `errors.Is` and `errors.As`**, never with `==` or by matching strings. (`errorlint`)
- **Error strings** are lowercase with no trailing punctuation, because they're joined into longer messages. Messages
  shown to the user are written separately as full sentences, in the command or the desktop app, not in
  `internal/vita`.
- **No `panic`** except for a programming error that can't happen when the code is correct, such as an embedded asset
  that fails to parse. Never panic on user input, file contents, or system call results.
- **Exit codes** for the command are documented in the README, and each one has a test.

---

## 5. Testing and coverage

### 5.1 Coverage: 90% minimum

- **Every package must have at least 90% statement coverage, and so must the repository as a whole.** A pull request
  that drops any package below 90% fails CI. The per-package gate stops one well-tested package from hiding an
  untested one.
- **Measured across all three operating systems.** CI runs the tests with `-coverprofile` on Linux, Windows, and
  macOS, merges the three profiles, and checks the merged result. This way `desktop_windows.go` counts the Windows run
  and `desktop_linux.go` counts the Linux run.
- **Each package's tests cover that package.** Coverage from another package's tests (`-coverpkg`) doesn't count.
- **Exclusions are rare and listed.** The only code that may be excluded is a thin adapter that calls something CI
  can't run, such as a native file dialog that needs a real display. It must not contain branching logic: move the
  logic into a tested file. Each excluded file is listed in `.github/coverage-exclusions.txt` with a one-line reason,
  and changes to that list go through CODEOWNERS review like any other file. Generated code is excluded
  automatically.
- **Coverage is the floor, not the goal.** A test that runs code without checking its result doesn't count in
  review. Every test asserts on behavior.

### 5.2 How tests are written

- **Table-driven** tests for anything with more than two cases. Shared setup goes in helpers that call `t.Helper()`.
- **Hermetic.** No network, no real home folder, no fixed paths. Use `t.TempDir()`, `t.Setenv()`, and
  `t.Context()`. Tests must pass with `-race`, in any order, with `-count=2`.
- **No sleeps.** Wait on channels, use injected clocks, or poll with a deadline.
- **Fuzz the parsers.** Every function that parses outside input (keys, recipients, file names, SVG paths, ICO files)
  has a `Fuzz` test with a seed corpus in `testdata/`.
- **Golden files** live in `testdata/` and are regenerated with `go test ./... -update`, never by hand.
- **Every bug fix comes with a test** that fails without the fix.
- **The desktop app is tested headless.** Screens are laid out into a Gio `op.Ops` in tests and checked through their
  state and events. No window is needed.
- **Interop** with the reference `age` CLI stays tested both ways for every key type.

---

## 6. Dependencies and GitHub Actions

This section replaces the dependency policy in the README; the README links here.

- **Always the latest.** Every Go module in the build graph, the `go` directive in `go.mod`, the Go toolchain used in
  CI and releases, every GitHub Action, and every development tool stay on their latest release.
- **Updated weekly by Dependabot,** with all Go modules in one pull request, all actions in another, and all tools in
  a third. A 7-day cooldown applies to version updates, but not to security updates. A Dependabot pull request is
  merged within a week of being opened, once CI passes. If an update breaks something, fix the code; don't pin the
  old version.
- **Actions are pinned to a full commit SHA**, with the version in a comment: `uses: actions/checkout@<sha> # v7.0.1`.
  Never pin to a tag or branch.
- **Development tools** are declared with `tool` directives in modules of their own, so Dependabot updates them and
  they stay out of the app's module graph: `golangci-lint` (which includes `gofumpt`) and `govulncheck` in
  `tools/go.mod`, and `actionlint` in `tools/actionlint/go.mod`, because it needs an older release of a library than
  golangci-lint does. Run them with `go tool -modfile=<that go.mod> <name>`. They're never installed by version
  strings in YAML.
- **`govulncheck`** runs in CI on every pull request, and weekly on a schedule against `main`.
- **`go mod verify`** and `go mod tidy -diff` run in CI.
- **No `replace` directives** in `go.mod`.
- **Adding a dependency** (not updating one) needs the maintainer's approval. The pull request has to say why the
  standard library or an existing dependency won't do, and confirm that the license is compatible with Apache-2.0 and
  that the project is actively maintained.
- **Workflows** use least-privilege `permissions:` (read-only at the top, write granted per job), have a
  `timeout-minutes` on every job, use `persist-credentials: false` on checkout, and never use `pull_request_target`
  or `workflow_run` to run pull request code.

---

## 7. Security

This is a tool that handles private keys. These rules apply on top of everything above.

- **No custom cryptography.** All encryption, decryption, and key handling goes through `filippo.io/age`.
- **Secrets are never logged, printed, or put in error messages.** That covers private keys and decrypted contents.
  File paths and fingerprints are fine.
- **Private key files are written with mode `0600`** and are never overwritten. Every output file is written to a
  temporary file and then published without overwriting anything (the existing `publish` pattern).
- **No network code in the build graph.** The existing tests enforce this for the command and the desktop app, and
  they stay.
- **Validate at the boundary.** Input from files, the clipboard, dropped files, and command-line arguments is
  validated where it enters the program. Code further in can trust it.
- **Reproducible builds** remain required: the release workflow builds twice and compares checksums.
- `gosec` runs in CI. Every finding is either fixed or suppressed with a reason (see [section 10](#10-exceptions)).

---

## 8. Comments and documentation

- **Every package has a package comment** and **every exported identifier has a doc comment** that starts with its
  name. (`revive`)
- **Comments explain why**, not what. If a comment is needed to explain what code does, rename or restructure the
  code first.
- **No commented-out code.** Git history keeps the old version.
- **`TODO` comments reference an issue:** `// TODO(#42): …`. A TODO without an issue fails review.
- **User-facing behavior changes update the README** in the same pull request.

---

## 9. Git and pull requests

- **Conventional Commits** for commit messages and pull request titles: `feat:`, `fix:`, `refactor:`, `test:`,
  `docs:`, `ci:`, `chore:`, with an optional scope (`feat(gui):`). Use the imperative mood.
- **Pull requests are small and do one thing.** Refactors, behavior changes, and dependency updates are separate
  pull requests.
- **All changes reach `main` through a pull request** that passes every CI gate and has CODEOWNERS approval. Squash
  merge only.
- **Release tags are signed** (`git tag -s`) and match `vX.Y.Z` or `vX.Y.Z-prerelease`. The release workflow already
  enforces this.

---

## 10. Exceptions

- A lint suppression names the linter and gives a reason: `//nolint:funlen // one switch over all 14 SVG path
  commands reads better than 14 helpers`. A bare `//nolint` fails CI. (`nolintlint`)
- A coverage exclusion follows [5.1](#51-coverage-90-minimum).
- To change a standard, open a pull request against this document. Don't make exceptions in code review.

---

## CI gates

Every pull request and every push to `main` must pass all of these. "All" means Linux, Windows, and macOS.

| Gate | Runs on | Command |
|------|---------|---------|
| Module tidy and verified | Linux | `go mod tidy -diff`, `go mod verify` (for the app and both tool modules) |
| Vet | All | `go vet ./...` |
| Lint and formatting | All | `golangci-lint run` with the linters and formatters below |
| Tests with race detector | All | `go test -race -count=1 -coverprofile=… ./...` |
| Coverage | Linux, after the test jobs | `go run ./scripts/coverage` merges the three profiles; every package and the total ≥ 90% |
| Vulnerabilities | Linux, and weekly on `main` | `govulncheck ./...` |
| Workflows | Linux | `actionlint` |
| Fuzz smoke | Linux | each `Fuzz` test for 10 seconds |

**golangci-lint linters:** `varnamelen` (configured to the [allowlist](#13-allowed-short-names-and-abbreviations), with
receivers and return values checked), `revive` (exported docs, receiver consistency), `staticcheck` (including
initialism checks), `errcheck`, `errorlint`, `gosec`, `funlen`, `gocognit`, `nestif`, `lll`, `dupl`, `goconst`,
`exhaustive` (every `switch` over an enum such as `screen` or `buttonKind` handles every value), `unparam`, `unused`,
`ineffassign`, `misspell`, `predeclared`, `forcetypeassert`, `nakedret`, `nolintlint`; formatters `gofumpt` and
`goimports`. The configuration lives in `.golangci.yml`. `gosec`'s G304 check (opening a file whose path is a
variable) is turned off there, because opening the files the user chose is what these programs do.

Linters can't fully check N4, N5, N9, N10, and N12 (verb phrases, boolean naming, vocabulary, length that follows
scope, pointless wrappers). Reviewers check those.

---

## Rolling this out

**Phase 1** wrote the rules down. **Phase 2** brought the code and CI in line, on one branch: the tooling and CI gates
first (with the coverage gate at 90% from the start), then one commit series per package that renamed everything to
section 1, split platform files into tested logic and thin adapters, and added tests. Phase 2 is finished when every
gate passes on Linux, Windows, and macOS.

### State when adopted (2026-11-23)

**Coverage** (Linux profile only, before merging Windows and macOS; exclusions applied):

| Package | Before | After |
|---------|--------|-------|
| `cmd/vita-decrypt-gui` | 12.6% | 98.7% |
| `internal/desktop` | 15.2% | 91.3% |
| `internal/vita` | 82.2% | 93.5% |
| `scripts/coverage` | (new) | 99.2% |
| `scripts/winres` | 85.7% | 99.2% |
| root (command) | 88.7% | 98.6% |
| **Total** | **28.1%** | **97.9%** |

**Coverage exclusions:** three files, all in `internal/desktop`: the Windows COM and Win32 adapter, the macOS AppKit
adapter, and its cgo callbacks. See `.github/coverage-exclusions.txt`.

**Lint suppressions:** 23, each with a reason. 21 are `gosec`: 19 in the platform adapters and their test fakes
(pointers and sizes passed to Windows, and running the Linux file chooser and file manager), plus the screenshot
folder in a test and the `.syso` file's permissions in `scripts/winres`. The other 2 are `dupl` on the light and
dark palettes, which are parallel tables by design.

**Renamed exports in `internal/vita`:** `ValidName` → `IsValidName`, `OutputName` → `DecryptedName`, `KeyFileName` →
`PrivateKeyFileName`, `ErrCannotOpen` → `ErrCannotDecrypt`, `ErrBadName` → `ErrInvalidName`, and `KeyPair.Path` /
`KeyPair.PublicPath` → `PrivateKeyPath` / `PublicKeyPath`.

**Not yet run on real Windows or macOS:** native file dialogs, drag and drop, and "show in folder". The tests drive
their logic through fakes; the thin adapters that make the native calls need a desktop session.

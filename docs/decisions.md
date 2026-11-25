# Decisions

The main choices behind vita-decrypt, why they were made, and what they cost. Add an entry when a decision changes
how the project is built or what it promises; don't rewrite an old one. If a decision is reversed, add a new entry
that says so and mark the old one **Replaced by D*n***.

Each entry gives the decision, the reasons, and the trade-offs.

---

## D1. Go, with static binaries

**Decision:** write both programs in Go.

**Why:** age's reference implementation, `filippo.io/age`, is Go, so the tool uses the same code the age CLI does
rather than a port. Go builds static binaries with no runtime to install, cross-compiles the command for every
platform from one machine, and makes reproducible builds straightforward.

**Trade-off:** Go's naming conventions are terse; the [coding standards](coding-standards.md) override them (D10).

## D2. All cryptography through age

**Decision:** use age (v1 format, `filippo.io/age`) for every encryption, decryption, and key operation, and write no
cryptographic code here.

**Why:** Vita seals with age, so the files must be age files. age is small, audited, and widely implemented, and its
files open with the age CLI if this tool disappears.

**Trade-off:** features age doesn't have (passphrase-wrapped keys, signatures) aren't available.

## D3. Hybrid post-quantum keys by default

**Decision:** `keygen` and the app create ML-KEM-768 + X25519 hybrid keys (`age1pq1…`) unless asked for X25519.

**Why:** files sealed today may be stored for years. A hybrid key stays safe if either half holds, including against
a future quantum computer that breaks X25519.

**Trade-off:** hybrid keys need age v1.3.0 or later. `--classic` keeps X25519 available for older tools.

## D4. Offline, enforced by tests

**Decision:** neither program contains network code, and tests fail if a network package enters either build.

**Why:** the whole point is that the private key stays on the customer's machine. Proving the absence of network code
is easier than reviewing what network code does, and a test keeps proving it as dependencies change.

**Trade-off:** no update checks, crash reports, or online help. Updates are announced through releases.

## D5. Never overwrite; publish only authenticated output

**Decision:** write every output to a temporary file in the output folder, and publish it with a hard link (or an
exclusive create and copy) only once it's complete and, for decryption, fully authenticated.

**Why:** overwriting a key file loses access to everything sealed to it, and partial plaintext from a tampered file is
dangerous. Both failures are silent if they happen, so the code makes them impossible.

**Trade-off:** to replace an output, the person deletes the old file first.

## D6. One shared package, two front ends

**Decision:** put everything the command and the app do in `internal/vita`; keep `main.go` and
`cmd/vita-decrypt-gui` to input, output, and presentation.

**Why:** the command and the app must behave identically (same names, same errors, same refusals), and the security
properties should be tested once.

## D7. Gio for the desktop app

**Decision:** build the desktop app with [Gio](https://gioui.org) and draw every screen in the app's own code.

**Why:** Gio is pure Go, cross-compiles for Windows without cgo, has no network code, and gives full control of
drawing, so the app can match Vita's design (D13) exactly instead of looking like a stock widget kit.

**Trade-offs:** Gio has no Linux drag and drop (the app takes pasted files instead), sets no window icon on Linux
(that needs a `.desktop` file), and needs cgo on Linux and macOS. Native file dialogs are written per platform in
`internal/desktop`.

## D8. Reproducible builds from signed tags

**Decision:** release only from a signed `v*` tag on `main`. The release workflow builds everything twice, compares
checksums, and creates a draft that a maintainer publishes.

**Why:** people trusting this tool with private keys should be able to check that the binary matches the source.

**Trade-off:** the macOS desktop app isn't released until it can be signed and notarized.

## D9. Windows resources without external tools

**Decision:** `scripts/winres` writes the Windows icon and version info itself, byte for byte as `windres` would.

**Why:** release builds stay reproducible and need nothing beyond Go, and the release version can be stamped in at
build time.

## D10. Our own coding standards over Go conventions

**Decision:** adopt the [coding standards](coding-standards.md) (2026-11-23), which choose descriptive names over Go's
short-name habits where they conflict, while staying brief and avoiding repetition.

**Why:** the code is read far more than it's written, and by people who don't write Go every day. `p`, `s`, and
`r` save typing once and cost understanding every time.

**Trade-off:** the code reads differently from typical Go. `varnamelen` and `revive` enforce the rules so they don't
depend on memory.

## D11. 90% coverage, merged across operating systems

**Decision:** every package and the total need 90% statement coverage, measured by merging the Linux, Windows, and
macOS profiles. Only thin adapters that make system calls can be excluded, each with a reason.

**Why:** much of the risky code is platform-specific; measuring one platform would either ignore it or force it to
be excluded. Per-package checks stop a well-tested package from hiding an untested one.

**Trade-off:** platform code is split into tested logic and thin adapters, which means more files and some injected
functions.

## D12. Development tools in their own modules

**Decision:** declare golangci-lint and govulncheck with `tool` directives in `tools/go.mod`, and actionlint in
`tools/actionlint/go.mod`.

**Why:** tool versions are pinned and updated by Dependabot like any other dependency, without entering the app's
module graph. actionlint needs a different version of a YAML library than golangci-lint, so it gets its own module.

## D13. Follow Vita's design system

**Decision:** the desktop app uses Vita's color tokens, fonts (Fraunces and Inter), radii, and accessibility
rules. See [design](design.md).

**Why:** customers move between Vita and this app when they set up sealing and when they open files. It should look
and feel like the same product.

**Trade-off:** design changes start in Vita's design system and are mirrored here by hand.

## D14. Fingerprints shared with Vita

**Decision:** fingerprints are the first 8 bytes of the SHA-256 of the canonical public key, as four groups of four
hex digits, tested against vectors shared with Vita. See [file formats](file-formats.md#fingerprints).

**Why:** a person can compare 16 characters by eye, and the comparison only means something if both sides compute
the same value.

**Trade-off:** changing the format means changing Vita and this tool together.

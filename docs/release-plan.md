# Release Plan

Scaffolded 2026-11-22 from an earlier prototype. No published release yet.

## Done

- `keygen` (hybrid by default, `--classic` for X25519; writes the private and public key files), `decrypt` (temp
  file, then a no-overwrite link), `seal` (to test a key pair; same no-overwrite publish), and `fingerprint`, on
  `filippo.io/age` v1.3.2.
- Tests: round trips for both key types (decrypt, and seal then decrypt), tampering, wrong key, no overwrite, name rule, exit codes, the shared
  fingerprint vectors, and no network packages in the build graph.
- Reproducible static builds for five targets (`scripts/build.sh`), checked by building twice and comparing
  checksums. Keys and files interoperate with the age v1.3.2 CLI both ways.
- Desktop app (`cmd/vita-decrypt-gui`, Gio v0.10.3): open, create, check, and seal screens, light and dark
  themes, About dialog. Public key fields have Paste and "Choose file…" buttons; fingerprints have Copy. Shares `internal/vita` with the command. Native file choosers
  (Windows IFileOpenDialog, macOS NSOpenPanel, Linux desktop portal or zenity), dropped files on Windows and macOS,
  and pasted files everywhere. Its own test keeps HTTP, TLS, and other network packages out of every platform's
  build; only `net/url`, `net/netip`, and `net` as used by the D-Bus client and `golang.org/x/sys/windows` are
  allowed.
- License: Apache-2.0 (`LICENSE`, `NOTICE`). Security policy: `SECURITY.md`.
- Dependencies updated to their latest versions on 2026-11-22 (Go 1.26 minimum, required by the `golang.org/x`
  modules); see the README's dependency policy.
- CI (`.github/workflows/ci.yml`): the gates in [the coding standards](coding-standards.md#ci-gates) (lint,
  vet, race-detector tests, 90% coverage merged across Linux, Windows, and macOS, govulncheck, actionlint, fuzzing)
  for every push to main and pull request. Release (`release.yml`): on a `v*` tag, runs CI, builds `scripts/build.sh` twice on
  Ubuntu 22.04 and compares checksums, and creates a draft GitHub release with the license notices; it doesn't
  include the macOS desktop app. Dependabot proposes Go module and action updates weekly, after a 7-day cooldown.
- Release hardening: the release only runs for a signed `v*` tag on a commit that's on main; only the maintainer can
  create `v*` tags; `CODEOWNERS` names the maintainer for every file. Once the repo is public, add the maintainer as
  a required reviewer on the `release` environment.
- Windows icon and version info for both programs (`scripts/winres`, tested byte for byte against windres's
  output); release builds stamp the release version.

## Before the first release (owner decisions)

- **Repository:** create `vita-workflow/vita-decrypt`, push, and make it public, with private vulnerability
  reporting turned on (`SECURITY.md` points to it). The module path already assumes this name.
- **Signing:** how releases are signed (for example Sigstore/cosign keyless from CI, or minisign) and where the
  checksums and signatures are published.
- **Desktop app testing:** the macOS code (cgo, Objective-C) hasn't been compiled or run, and the Windows file
  chooser and drop target haven't been tried on real Windows. Both need a pass on real machines. On Linux, X11
  sessions on some NVIDIA drivers fail to create the EGL surface (Gio supports only EGL there); Wayland works.
- **Desktop app packaging:** a macOS `.app` bundle with signing and notarization, and Linux packaging (a `.desktop`
  file at least). Release builds need a Linux and a macOS runner because those builds use cgo.
- **Font and icon notices:** ship the OFL texts (`cmd/vita-decrypt-gui/fonts/*-OFL.txt`) and the Lucide license
  (`cmd/vita-decrypt-gui/icons/LICENSE`) with the release, next to this tool's own license.
- **Guide:** the recipient guide and the download page that Vita links to.
- **External cryptographic review:** Vita's sealing design and this tool are reviewed together before any
  production security claim.

# vita-decrypt

An offline tool for files that Vita sealed. Vita's File protection with sealing encrypts approved files with
[age](https://age-encryption.org/v1) to a key pair whose private key Vita never holds. This tool creates those key
pairs and opens the sealed files. It has no network code, so a private key never leaves your machine.

## Commands

```sh
vita-decrypt keygen --name FamilyOnly            # hybrid post-quantum key pair (ML-KEM-768 + X25519)
vita-decrypt keygen --name FamilyOnly --classic  # X25519 key pair, for older age tools
vita-decrypt decrypt FamilyOnly-private-key.txt report.pdf.age
vita-decrypt seal FamilyOnly-public-key.txt test.pdf     # seal a file the way Vita does, to test a key pair
vita-decrypt fingerprint FamilyOnly-public-key.txt       # the fingerprint Vita shows for a public key
```

- `keygen` writes `NAME-private-key.txt` (mode 0600 on Linux and macOS; on Windows it gets the folder's permissions)
  and `NAME-public-key.txt`, and prints the public key and its fingerprint. It never overwrites either file. Paste the
  public key into Vita. Keep the private key file safe and backed up: **Vita can't recover it, and without it sealed
  files can't be opened.** Names follow Vita's rule: 1–40 letters, digits, or hyphens, starting with a letter or
  digit.
- `decrypt` writes the file next to the `.age` file, without the suffix. It never overwrites an existing file, and a
  damaged or changed file leaves nothing behind.
- `seal` writes `FILE.age` next to the file, sealed to the public key as Vita seals files, so a new key pair can be
  tested end to end: seal a test file, then `decrypt` it with the private key. It never overwrites a file.
- `seal` and `fingerprint` take the public key itself (`age1...`) or the public key file. The public key file is also
  an age recipients file (`age -R`).
- Check that the fingerprint Vita shows for a key matches the one `keygen` printed before sealing with it.

Exit codes: `0` when the command succeeded, `1` when it failed (for example a wrong key, a damaged file, or a file
that already exists), and `2` for a usage error (an unknown command or flag, or a missing argument).

## Desktop app

`vita-decrypt-gui` (in `cmd/vita-decrypt-gui`) does the same four jobs in a window, for people who'd rather not use
a terminal: open sealed files with a private key file, create a key pair, check a fingerprint, and seal a test file.
Like the command it has no network code and never overwrites a file. Public key fields have Paste and "Choose
file…" buttons, so nobody has to type a key, and fingerprints have a Copy button.

- **Windows and macOS:** drop the private key file and the `.age` files on the window, or choose them with the
  system file chooser. On the Seal screen, a dropped file is the file to seal.
- **Linux:** the window can't accept dropped files (Gio, the UI toolkit, doesn't support it yet), so copy the files
  in your file manager and press Ctrl+V in the window, or use the file chooser (the desktop portal, or `zenity`).

Pasting full file paths works everywhere too (Ctrl+V, or Cmd+V on macOS), such as from Windows Explorer's "Copy as
path" or Finder's Option-Command-C: a pasted `.age` file is added to the list and any other file is read as the
private key. Files copied with a plain Copy in Explorer or Finder don't paste, because those put no paths on the
clipboard as text; drop them instead.

## Without this tool

Sealed files are standard age files, so the official [age](https://github.com/FiloSottile/age) CLI (v1.3.0 or later
for hybrid keys) opens them with the same private key file:

```sh
age --decrypt -i FamilyOnly-private-key.txt -o report.pdf report.pdf.age
```

## Building

Go 1.26 or later; the desktop app also needs cgo and graphics headers on Linux and macOS.

```sh
go test ./...               # tests, including the shared fingerprint vectors and the no-network checks
scripts/build.sh 1.2.3      # reproducible release files in dist/, with SHA256SUMS
```

[Development](docs/development.md) covers setup for each platform, the checks CI runs, fuzzing, screenshots of the
desktop app, and the Windows icon.

The app bundles the Fraunces and Inter fonts (SIL Open Font License 1.1, in `cmd/vita-decrypt-gui/fonts`) and Lucide
icons (ISC, in `cmd/vita-decrypt-gui/icons`).

## Releasing

Tag a commit on main with a signed tag and push it:

```sh
git tag -s v1.0.0 -m v1.0.0
git push origin v1.0.0
```

The release workflow checks that the tag is signed and on main, runs CI, builds the release files twice and compares
their checksums, and creates a draft release. Review the draft and publish it; a published release can't be changed.

## Contributing

Follow the [coding standards](docs/coding-standards.md). They cover naming, testing (at least 90% statement coverage
for every package), dependencies, and the checks CI runs. Before opening a pull request, run the checks in
[development](docs/development.md#checking-your-work).

Every dependency, GitHub Action, and development tool stays on its latest release; the
[dependency policy](docs/coding-standards.md#6-dependencies-and-github-actions) explains how.

## Security

See [SECURITY.md](SECURITY.md) to report a vulnerability privately. The [security model](docs/security-model.md)
explains what the tool protects and what it doesn't.

## Documentation

| Document | What it covers |
|----------|----------------|
| [Architecture](docs/architecture.md) | The programs and packages, how a file moves through decrypt and seal, the desktop app's structure, platform code, and staying offline. |
| [Design](docs/design.md) | How the desktop app looks: Vita's colors, type, shapes, icons, components, and accessibility rules. |
| [Security model](docs/security-model.md) | What's protected, what it relies on, and what's out of scope. |
| [File formats](docs/file-formats.md) | Key files, public keys, fingerprints, sealed files, and key pair names. |
| [Development](docs/development.md) | Setting up, running the checks, fuzzing, screenshots, the Windows icon, and building release files. |
| [Coding standards](docs/coding-standards.md) | Naming, structure, errors, testing and coverage, dependencies, security, and CI. |
| [Decisions](docs/decisions.md) | Why Go, age, Gio, hybrid keys, offline-only, and the other main choices. |
| [Release plan](docs/release-plan.md) | What must happen before the first release. |

[docs/README.md](docs/README.md) suggests where to start.

## License

[Apache License 2.0](LICENSE). Copyright 2026 TAnderson Enterprises LLC, doing business as Vita Workflow; see
[NOTICE](NOTICE). The bundled fonts and icons keep their own licenses, listed above.

## Status

Not released. See [the release plan](docs/release-plan.md) for what must happen before the first release.

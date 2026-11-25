# Architecture

How vita-decrypt is put together: the programs, the packages they share, how a file moves through them, and the
rules that keep the structure honest. For why it's built this way, see [decisions](decisions.md).

## Programs and packages

```
vita-decrypt (main.go)              vita-decrypt-gui (cmd/vita-decrypt-gui)
  command line                        Gio window: Open, Create, Check, Seal screens
        │                               │                    │
        └──────────────┬────────────────┘                    │
                       ▼                                     ▼
               internal/vita                          internal/desktop
   decrypt, seal, key pairs, fingerprints     file dialogs, show in folder, dropped
            (filippo.io/age)                   and pasted files (per operating system)
```

| Package | What it does | Depends on |
|---------|--------------|------------|
| `.` (root) | The `vita-decrypt` command: parses arguments, calls `internal/vita`, prints the result, and returns an exit code (0, 1, or 2). | `internal/vita` |
| `cmd/vita-decrypt-gui` | The desktop app. Draws every screen itself with Gio and runs the work off the UI thread. | `internal/vita`, `internal/desktop`, Gio |
| `internal/vita` | Everything both programs share: `Decrypt`, `Seal`, `CreateKeyPair`, `ReadPrivateKey`, `ReadPublicKey`, `Fingerprint`, and the naming rules for key and sealed files. No user interface code. | `filippo.io/age` |
| `internal/desktop` | What the desktop app needs from the operating system: `OpenFile`, `OpenFiles`, `ChooseFolder`, `Reveal`, `AttachDrop`, and `PathsFromText` for pasted paths. | `golang.org/x/sys`, a D-Bus client on Linux, cgo and AppKit on macOS |
| `scripts/coverage` | The CI coverage gate: merges the Linux, Windows, and macOS profiles and checks 90% per package. | standard library |
| `scripts/winres` | Writes the Windows icon and version info as a `.syso` file that `go build` links in. | standard library |

`internal/` keeps the shared packages private to this module. Nothing outside the repository can import them, so
their APIs can change with the programs that use them.

## How a file moves through

### Decrypt

1. The command or app reads the private key file with `vita.ReadPrivateKey`. It accepts at most 1 MiB and returns
   the age identities in it.
2. `vita.Decrypt` works out the output name (the input without `.age`, in the same folder) and refuses at once if a
   file is already there.
3. age reads the header and checks the identities against its recipients. A wrong key fails here with
   `ErrCannotDecrypt` before anything is written.
4. The plaintext streams into a temporary file (`.vita-decrypt-*`) in the output folder. age authenticates each
   64 KiB chunk as it goes. A damaged or changed chunk stops the copy with `ErrDamaged`.
5. Only after the last chunk is authenticated is the temporary file *published* at the output name: a hard link,
   which fails rather than replace a file that appeared in the meantime. File systems without hard links (FAT32,
   exFAT, some network shares) get an exclusive create and a copy instead.
6. The temporary file is always removed, whether the decryption worked, failed, or was cancelled.

### Seal

`vita.Seal` follows the same path in reverse: it parses the public key (hybrid `age1pq1…` or X25519 `age1…`),
encrypts into a temporary file, and publishes `FILE.age` without overwriting anything. It exists so a new key pair can
be tested end to end; Vita does the real sealing.

### Create a key pair

`vita.CreateKeyPair` checks the name against Vita's rule, generates a hybrid post-quantum key pair (ML-KEM-768 +
X25519) or, with `--classic`, an X25519 one, and writes `NAME-private-key.txt` (mode 0600) and `NAME-public-key.txt`
(mode 0644). If either file exists, nothing is written.

### Fingerprint

`vita.Fingerprint` is a pure function of the public key text; see [file formats](file-formats.md#fingerprints).

## The desktop app

- **One goroutine owns the UI.** `App` is only touched by the Gio event loop. Work that takes time (decrypting,
  sealing, creating keys, waiting on a file dialog) runs in the background and hands its result back with
  `app.post`, which queues it for the event loop and asks for a redraw.
- **The outside world is injected.** `App` reaches the operating system only through a `system` struct (file dialogs,
  reveal, background work) and a `window` interface. `main.go` passes the real ones; the tests pass fakes and drive
  every screen headlessly through a real Gio input router, with no GPU or display.
- **Screens are files.** Each screen has its state and logic in one file and its layout in another: `open.go` and
  `openview.go`, `create.go` and `created.go`, `seal.go` and `sealview.go`, and `check.go`. Shared pieces live in
  `buttons.go`, `fields.go`, `widgets.go`, `containers.go`, `messages.go`, `sidebar.go`, and `about.go`.
- **Drawing is done here, not by a widget kit.** `draw.go`, `text.go`, `icons.go`, and `svgpath.go` draw boxes, text,
  and the Lucide icons (parsed from their SVG paths at start-up) to match the [design](design.md) exactly.
- **Theme choice is remembered** in `vita-decrypt/theme` under the user's config folder.

## Platform code

Code that only runs on one operating system is split in two (see the
[coding standards](coding-standards.md#51-coverage-90-minimum)):

- **Logic** that decides something: parsing dropped paths, building dialog filters, handling COM reference counts,
  choosing between the desktop portal and zenity. It lives in files the tests reach on every platform, or on their
  own platform's CI run.
- **Thin adapters** that only make the system call: `desktop_windows.go` (COM and Win32), `desktop_darwin.go` and
  `desktop_darwin_export.go` (cgo into AppKit). They have no branching logic and are the only files excluded from
  the coverage gate.

| | Windows | macOS | Linux |
|---|---|---|---|
| File chooser | `IFileOpenDialog` | `NSOpenPanel` | desktop portal over D-Bus, then `zenity` |
| Show in folder | Explorer, selected | Finder (`open -R`) | file manager over D-Bus, then `xdg-open` on the folder |
| Dropped files | `IDropTarget` | `NSView` drag destination | not supported by Gio; paste instead |
| Pasted paths | yes | yes | yes, including file manager copies |

## Staying offline

Neither program has network code, so a private key or plaintext can't leave the machine through it. Tests enforce
this rather than trusting review:

- `TestNoNetworkPackages` (root) fails if any `net`, `net/…`, or `crypto/tls` package is in the command's build
  graph.
- The desktop app's network test does the same for every platform's build. Its only exceptions are `net/url` and
  `net/netip` (parsing), and `net` where the Linux D-Bus client and `golang.org/x/sys/windows` need it for local
  sockets and types.

## Build and release

- `scripts/build.sh VERSION` builds reproducible binaries into `dist/` with a `SHA256SUMS` file. The command is
  static (no cgo) for every target; the desktop app cross-compiles for Windows, and needs cgo on Linux and macOS.
- `scripts/winres` writes the Windows resources for the release version before building.
- The release workflow runs on a signed `v*` tag on `main`, runs CI, builds twice and compares checksums, and creates
  a draft release. See [the release plan](release-plan.md) and [development](development.md).

## Related docs

- [Design](design.md): what the desktop app looks like and why.
- [Security model](security-model.md): what the tool protects and what it doesn't.
- [File formats](file-formats.md): key files, sealed files, fingerprints, and names.
- [Decisions](decisions.md): the choices behind this structure.

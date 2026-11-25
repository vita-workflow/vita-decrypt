# Security model

What vita-decrypt protects, what it relies on, and what it doesn't try to do. To report a vulnerability, follow
[SECURITY.md](../SECURITY.md); this document explains the design that policy protects.

## What it's for

Vita's "File protection with sealing" encrypts approved files with [age](https://age-encryption.org/v1) to a public
key the customer gives it. Vita only ever holds the **public** key, so Vita, and anyone who breaks into Vita, can seal
files but can't open them. vita-decrypt is the customer's side: it creates the key pair, keeps the private key on the
customer's machine, and opens the sealed files there.

## What it protects

| Asset | Protection |
|-------|------------|
| **The private key** | Created on the customer's machine and never sent anywhere: neither program has network code, and tests fail the build if any appears ([architecture](architecture.md#staying-offline)). On Linux and macOS the file is mode `0600`; on Windows it takes the folder's permissions. It's never printed, logged, or put in an error message. |
| **The plaintext** | Written only after age has authenticated every chunk of the sealed file, so a damaged, truncated, or changed file, or the wrong key, leaves nothing behind. Decrypted files are created with mode `0600` on Linux and macOS. |
| **Existing files** | Never overwritten: not a key file, not a decrypted file, not a sealed file, even if one appears while the work is running. Outputs are published with a hard link or an exclusive create, both of which fail rather than replace a file. |
| **The right key** | Fingerprints let a person check that the public key Vita shows is the one they created before Vita seals with it. Both sides compute them the same way, checked by shared test vectors ([file formats](file-formats.md#fingerprints)). |
| **The build** | Release binaries are reproducible: the release workflow builds twice and compares checksums, and anyone can rebuild from the tag and compare. Releases run only from a signed tag on `main`. |

## What it relies on

- **age and `filippo.io/age`** for all cryptography. This repository contains no cryptographic code of its own, and
  the [coding standards](coding-standards.md#7-security) forbid adding any.
- **The operating system** to enforce file permissions and to keep other users and programs out of the user's
  session.
- **The person** to keep the private key file backed up and safe. Vita can't recover it, and without it sealed files
  can't be opened. The app won't finish creating a key pair until the person confirms they've backed it up.

## Design choices that follow

- **Hybrid post-quantum keys by default.** New key pairs are ML-KEM-768 + X25519 (`age1pq1…`), so files sealed today
  stay safe against a future quantum computer. `--classic` makes an X25519 key for older age tools.
- **Offline by construction.** No update checks, telemetry, crash reports, or online help. The only network-adjacent
  code is the Linux desktop app's D-Bus client, which talks to the local session bus for the file chooser and file
  manager.
- **Temporary files stay next to the output** (`.vita-decrypt-*` in the same folder), so publishing never copies
  plaintext across drives, and they're removed when the work ends, fails, or is cancelled. Ctrl+C in the command
  cancels cleanly.
- **Input is checked at the edge.** Key files are capped at 1 MiB; public keys are parsed before use; file names are
  checked against Vita's rules; dropped and pasted paths are parsed by fuzz-tested code.
- **Errors explain without leaking.** Messages name the file and say what went wrong ("this key can't open the file",
  "the file is damaged or was changed; nothing was written") but never include key material or plaintext.

## What it doesn't protect against

- **Someone who controls the machine** or the user's account: they can read the private key file or the decrypted
  files. Full-disk encryption and an account password are the defense there.
- **A lost private key.** There's no recovery by design: a recovery path would be a way in.
- **Copies the person makes.** Decrypted files are ordinary files once written; where they go next is up to the
  person.
- **Temporary plaintext on a crash.** If the process is killed (not cancelled) mid-decryption, a `.vita-decrypt-*`
  file holding partial plaintext can stay in the output folder until it's deleted.
- **Vita itself.** The service's security is Vita's; this tool only assumes Vita seals to the public key it was given.

## Checks that keep it true

- Tests for tampering, truncation, wrong keys, existing outputs, and files that appear mid-operation.
- Round trips against age's own encryption and decryption, in both directions, for both key types. (Interoperability
  with the age CLI was checked by hand for v1.3.2; it isn't automated.)
- Fuzz tests for every parser of outside input, run in CI.
- `govulncheck` on every pull request and weekly, and `gosec` in the linter.
- The no-network tests for the command and for every platform's build of the desktop app.

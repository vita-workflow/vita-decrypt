# File formats

Every file vita-decrypt reads or writes is a standard age file or a plain text file age's own tools can read. Nothing
here is a private format, so the files stay usable [without this tool](../README.md#without-this-tool).

## Key pair names

Names follow Vita's rule, which `vita.IsValidName` checks: **1 to 40 ASCII letters, digits, or hyphens, starting with
a letter or digit** (`^[A-Za-z0-9][A-Za-z0-9-]{0,39}$`). Vita applies the same rule to the
key names it stores, so a name that passes here is accepted there.

## Private key file

`NAME-private-key.txt`, mode `0600` on Linux and macOS. It's an age identity file (`age -i` reads it):

```
# Vita key pair: FamilyOnly
# public key: age1pq1…
AGE-SECRET-KEY-PQ-1…
```

- The first line names the key pair; the app shows it when the file is opened. If the line is missing or the name in
  it is invalid, the name is taken from a `NAME-private-key.txt` file name instead, else there's no name.
- The second line records the public key, so it can be recovered from the private key file.
- The last line is the identity: `AGE-SECRET-KEY-PQ-1…` for a hybrid key, `AGE-SECRET-KEY-1…` for X25519 (`--classic`).
- When reading, any age identity file works, including one with several identities or none of these comments. Files
  over 1 MiB are refused.

## Public key file

`NAME-public-key.txt`, mode `0644`. It's an age recipients file (`age -R` reads it):

```
# Vita key pair: FamilyOnly
# fingerprint: CE94 50DF DFF5 9E9D
age1pq1…
```

When reading (`vita.ReadPublicKey`), blank lines and lines starting with `#` are skipped and exactly one public key
must remain. A file containing `AGE-SECRET-KEY-` is refused with "this is a private key file; choose the public key
file instead", so a private key can't be pasted into the wrong place by mistake. Files over 1 MiB are refused.

## Public keys

| Kind | Prefix | Created by |
|------|--------|-----------|
| Hybrid post-quantum (ML-KEM-768 + X25519) | `age1pq1` | `keygen` (the default), age v1.3.0 or later |
| X25519 | `age1` | `keygen --classic`, any age version |

Public keys are compared and fingerprinted in **canonical form**: surrounding whitespace trimmed and lowercased.

## Fingerprints

A fingerprint is a short value a person can compare by eye to check that the public key Vita shows is the one they
created. Vita and vita-decrypt compute it the same way:

1. Take the canonical public key (trimmed, lowercased).
2. SHA-256 its UTF-8 bytes.
3. Keep the first 8 bytes, write them as uppercase hex, and split them into four groups of four: `CE94 50DF DFF5 9E9D`.

`testdata/fingerprint-vectors.json` holds test vectors that both this repository and Vita check against,
covering X25519 and hybrid keys and a key with extra whitespace and uppercase letters. **Changing the rule means
changing it in Vita at the same time**, and adding vectors for the change to both.

## Sealed files

A sealed file is a binary age v1 file (not ASCII-armored) named `FILE.age`:

- **Decrypting** `report.pdf.age` writes `report.pdf` in the same folder. The input must end in `.age` and have a
  name before it; `vita.DecryptedName` checks this before any work starts.
- **Sealing** `report.pdf` writes `report.pdf.age` in the same folder, encrypted to one public key, as Vita does.
- While working, both write a temporary `.vita-decrypt-*` file in the output folder and remove it when they finish.

## Windows resources

`cmd/vita-decrypt-gui/rsrc_windows_amd64.syso` (and the command's copy, written at build time) is a COFF object that
holds the app icon and version information. `scripts/winres` writes it byte for byte as MinGW's `windres` would; see
[development](development.md#windows-icon-and-version-info).

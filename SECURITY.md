# Security policy

vita-decrypt handles private keys and the files sealed to them, so we take reports seriously and are grateful for
them.

## Reporting a vulnerability

Report privately through GitHub: open the repository's **Security** tab and choose **Report a vulnerability**. Please
don't open a public issue, pull request, or discussion for a security problem.

Include what you can of:

- the version or commit, and the operating system;
- what an attacker can do, and what they need first (for example a crafted `.age` file, or access to the machine);
- steps or a file that reproduce it.

We'll acknowledge the report, keep you informed while we work on a fix, and credit you when it's published unless you
ask us not to. Please give us a reasonable time to release a fix before you disclose the problem.

## Scope

In scope: this repository's command-line tool and desktop app, for example:

- a private key, or any plaintext, leaving the machine, or any network access at all (the tool has no network code by
  design);
- plaintext written for a file that is damaged, changed, or opened with the wrong key;
- an existing file or key being overwritten;
- a wrong fingerprint, or a public key wrongly accepted or rejected;
- private key files created with permissions that let other users read them;
- the release builds not matching this source.

Out of scope here:

- problems in [age](https://github.com/FiloSottile/age) or `filippo.io/age` itself: report those to age, following its
  own security policy;
- the Vita service: report those to Vita through its own channels;
- attacks that need control of the machine the tool runs on, or of the private key file.

## Supported versions

Fixes go into the latest release only. Until the first release, report against the `main` branch.

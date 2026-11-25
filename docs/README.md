# vita-decrypt documentation

Where to start:

- **Using the tool:** the [README](../README.md) covers the commands, the desktop app, and opening files without this
  tool.
- **Changing the code:** read [development](development.md) to set up and run the checks, the
  [coding standards](coding-standards.md) for the rules, and [architecture](architecture.md) for where things are.
- **Changing how the app looks:** [design](design.md). Design changes start in Vita's design system.
- **Reviewing its security:** the [security model](security-model.md) and [file formats](file-formats.md). Report
  vulnerabilities as [SECURITY.md](../SECURITY.md) describes.
- **Wondering why something is the way it is:** [decisions](decisions.md).

| Document | What it covers |
|----------|----------------|
| [architecture.md](architecture.md) | The programs and packages, how a file moves through each operation, the desktop app, platform code, staying offline. |
| [design.md](design.md) | The desktop app's look and feel, colors, type, shapes, icons, components, behavior, and accessibility. |
| [security-model.md](security-model.md) | What's protected and how, what the tool relies on, what's out of scope, and the checks that keep it true. |
| [file-formats.md](file-formats.md) | Key pair names, key files, public keys, fingerprints, sealed files, and Windows resources. |
| [development.md](development.md) | Setup, running and checking, fuzzing, screenshots, the Windows icon, release builds, and dependencies. |
| [coding-standards.md](coding-standards.md) | The rules for code in this repository, the CI gates, and the state when they were adopted. |
| [decisions.md](decisions.md) | The main decisions, why they were made, and what they cost. |
| [release-plan.md](release-plan.md) | What must happen before the first release. |

Keep these documents true: a pull request that changes what one describes updates it in the same pull request.

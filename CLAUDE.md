# vita-decrypt

The offline command and desktop app for files Vita seals with age. Start with the [docs index](docs/README.md).

## Merging pull requests

**Always merge with this command, never the GitHub web button:**

```sh
gh pr merge --squash --author-email 273693701+tandersonenterprises@users.noreply.github.com
```

Run it on the pull request's branch, or add the number after `merge`. The web button, and `gh pr merge` without
`--author-email`, credit the squash commit to the maintainer's primary email address, which then becomes public in
`main`'s history. Squash is the only merge method `main` allows.

Don't merge unless the maintainer asks you to.

## Rules

- **Never name Vita's private repositories, or their files, classes, or tests**, in code, docs, comments, commit
  messages, or pull request text. This repository is public. Say "Vita" or "Vita's design system".
- **Commits are signed** and authored as `Vita Workflow <273693701+tandersonenterprises@users.noreply.github.com>`.
  No other email address goes into a commit.
- **Everything reaches `main` through a pull request** that passes every CI check and has the code owner's approval.
  Branch first; never push to `main`.
- **Commit messages and pull request titles** use Conventional Commits (`feat:`, `fix:`, `docs:`, `ci:`, …).
- **Names are descriptive, not Go-terse.** The [coding standards](docs/coding-standards.md) override Go's habits
  where they conflict, and lint enforces them.
- **Coverage stays at 90% or more**, per package and in total, measured across Linux, Windows, and macOS.
- **Actions are pinned to a full commit SHA**, and only GitHub's own actions are allowed.

## Checking your work

Run what CI runs before opening a pull request (details in [development](docs/development.md)):

```sh
go vet ./...
go tool -modfile=tools/go.mod golangci-lint run ./...
go test -race -covermode=atomic -coverprofile=coverage.out ./...
go run ./scripts/coverage coverage.out
go tool -modfile=tools/go.mod govulncheck ./...
go tool -modfile=tools/actionlint/go.mod actionlint     # after changing a workflow
```

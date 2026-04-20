# CarWatch Project Rules

## Git Identity (overrides global)

This is a personal project. Always use the private GitHub account:

- **GitHub user:** Danielsio
- **Email:** kingddd301@gmail.com
- **Commit sign-off:** `Signed-off-by: Daniel Sionov <kingddd301@gmail.com>`

Before running any `gh` commands, ensure the active account is `Danielsio`:
```bash
gh auth switch --user Danielsio
```

## Commit Format (Conventional Commits)

PR titles and commit messages MUST use conventional commit prefixes.
CI uses these to auto-bump the version on merge to main:

- `fix:` → patch bump (1.0.X)
- `feat:` → minor bump (1.X.0)
- `feat!:` or body contains `BREAKING CHANGE` → major bump (X.0.0)
- `chore:`, `docs:`, `test:`, `refactor:`, `ci:` → patch bump

```
feat: add WhatsApp notification channel

Optional body here.

Assisted-By: Claude opus 4.6 <noreply@anthropic.com>
Signed-off-by: Daniel Sionov <kingddd301@gmail.com>
```

## Branch Creation

This is not a fork. Create branches from `main`:
```bash
git checkout -b my-branch-name main
```

## Pre-Push Checks

Always run the linter before pushing:
```bash
golangci-lint run ./...
```
Fix any issues before pushing. CI runs golangci-lint and will fail on lint errors.

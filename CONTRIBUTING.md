# Contributing

## Workflow

```
main          ← production (stable releases only)
└── dev       ← integration (all features merged here first)
    └── feat/your-feature  ← your daily work
```

## Just for you to know
- **Branch protection is enabled**: Direct pushes to `main` and `dev` are forbidden. You must open a Pull Request.
- A PR requires **1 approvals** before it can be merged
- All review **conversations must be resolved** before merging
- If you push a new commit on a PR, **previous approvals are invalidated** — reviewers must re-approve

---

## Branch Naming

```
<type>/<short-description>
```

Use lowercase and hyphens, no spaces or special characters.

| Type | Pattern |
|------|---------|
| Feature | `feat/<description>` |
| Bug fix | `fix/<description>` |
| Documentation | `docs/<description>` |
| Refactor | `refactor/<description>` |
| Chore | `chore/<description>` |
| Test | `test/<description>` |

Just like this: `feat/user-authentication`

---

## Naming Conventions

| Item | Convention | Example |
|------|------------|---------|
| Variables | `lowerCamelCase` | `userProfile := ...` |
| Constants | `lowerCamelCase` or `PascalCase` | `const maxRetries = 3` |
| Exported identifiers | `PascalCase` | `func FetchDefaults(...)` |
| Unexported identifiers | `lowerCamelCase` | `func fetchStars(...)` |
| Interfaces | `PascalCase`, noun or `-er` suffix | `type RoundTripper interface` |
| Structs | `PascalCase` | `type GitConfig struct` |
| Error variables | `err` prefix | `var errNotFound = errors.New(...)` |

> Follow the [Effective Go](https://go.dev/doc/effective_go) naming guidelines.

---

## Commit Messages

Please refer to the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification.

```
<type>: <short description in lowercase>

[optional body — explain the why, not the what]
```

| Type | When to use |
|------|-------------|
| `feat` | A new feature |
| `fix` | A bug fix |
| `docs` | Documentation changes only |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `chore` | Dependency updates, config, build system |
| `test` | Adding or updating tests |
| `style` | Formatting — no logic change |

**Rules:**
- Use the **imperative mood**: `add`, `fix`, `update` — not `added`, `fixed`, `updated`
- Keep the first line **under 72 characters**
- If needed: use a second `-m` flag to add a body with a more detailed description
- Write in **English**

Just like this: `git commit -m "feat: add github token validation" -m "ensure the token has the required scopes before attempting API calls"`

---

## Code Comments

- Write all comments in **English**
- Use comments to explain **why**, not what (the code explains itself)
- Use `// TODO:` for things to be done later
- Use `// FIXME:` for known issues that need to be addressed
- Use `// NOTE:` for important clarifications

**Example:**
```go
// NOTE: we replace http.DefaultTransport instead of injecting a client
// because several call sites use &http.Client{Timeout: d} without an explicit Transport.
http.DefaultTransport = redirectRoundTripper

// TODO: add retry logic on transient GitHub API errors (429, 502, 503)
stars, err := fetchGithubStars(ctx, repo, token)
```

---

## Pull Requests

- One PR = one feature or fix — keep it focused
- Always target `dev`
- Fill in the PR description — summarize what changed and why
- Assign at least 1 reviewers before submitting
- The **author cannot approve their own PR**
- Once approved and all conversations resolved, I will merge your changes.

---

## Code Quality

Before opening a PR, make sure:

```bash
go build ./...        # must compile without errors
go test ./...         # all tests must pass
go vet ./...          # no vet warnings
```

If `staticcheck` is available:
```bash
staticcheck ./...     # no warnings
```

- Do **not** use `panic` outside of truly unrecoverable situations — return errors instead
- Wrap errors with context: `fmt.Errorf("fetch failed: %w", err)`
- Pass `context.Context` to any function that does I/O
- New logic must come with table-driven tests (`_test.go` alongside the source file)

---

## Getting Started

```bash
git clone git@github.com:slouowzee/kapi.git
cd kapi
git checkout dev
git checkout -b feat/your-feature
# ... do your work ...
go test ./...
git commit -m "feat: add your feature" -m "explain the why, not the what"
git push origin feat/your-feature
# then open a PR on GitHub targeting dev
```

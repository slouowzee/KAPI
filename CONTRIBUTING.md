# Contributing

KAPI is open source and open to everyone. Whether you're fixing a bug, adding a feature, or improving the docs — contributions are welcome.

## Workflow

```
main          ← production (stable releases only)
└── dev       ← integration (all features merged here first)
    └── feat/your-feature  ← your daily work
```

## Just for you to know

- **Branch protection is enabled from v1.0.0 onwards**: Direct pushes to `main` and `dev` are forbidden. You must open a Pull Request.
- A PR requires **the approval from KAPI's creator** before it can be merged
- All review **conversations must be resolved** before merging
- If you push a new commit on a PR, **previous approvals are invalidated** — reviewers must re-approve

---

## Branch Naming

```
<type>/<short-description>
```

Use lowercase and hyphens, no spaces or special characters.

| Type | Pattern |
|---|---|
| Feature | `feat/<description>` |
| Bug fix | `fix/<description>` |
| Documentation | `docs/<description>` |
| Refactor | `refactor/<description>` |
| Chore | `chore/<description>` |
| Test | `test/<description>` |

Just like this: `feat/welcome-screen`

---

## Naming Conventions

| Item | Convention | Example |
|---|---|---|
| Variables | `lowerCamelCase` | `screenWidth` |
| Constants | `UPPER_SNAKE_CASE` | `API_BASE_URL` |
| Functions | `camelCase` (verb prefix) | `fetchFrameworks()` |
| Structs | `PascalCase` | `type AppModel struct` |
| Interfaces | `PascalCase` | `type Registry interface` |
| Parameters | `lowerCamelCase` | `func search(query string)` |

> **Go exception**: constructor functions follow the Go standard — use `New()` or `NewXxx()` for exported constructors (e.g. `func New() App`, `func NewSearchScreen() Screen`).

---

## Commit Messages

Please refer to the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification.

```
<type>: <short description in lowercase>

[optional body — explain the why, not the what]
```

| Type | When to use |
|---|---|
| `feat` | A new feature |
| `fix` | A bug fix |
| `docs` | Documentation changes only |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `chore` | Dependency updates, config, build system |
| `test` | Adding or updating tests |
| `style` | Formatting, missing semicolons, etc — no logic change |

**Rules:**

- Use the **imperative mood**: `add`, `fix`, `update` — not `added`, `fixed`, `updated`
- Keep the first line **under 72 characters**
- If needed: use a second `-m` flag to add a body with a more detailed description
- Write in **English**

Just like this: `git commit -m "feat: add welcome screen" -m "first screen the user sees when launching KAPI"`

---

## Code Comments

- Write all comments in **English**
- Use comments to explain **why**, not what (the code explains itself)
- Use `// TODO:` for things to be done later
- Use `// FIXME:` for known issues that need to be addressed
- Use `// NOTE:` for important clarifications

```go
// NOTE: fallback is used when the remote API is unreachable
var fallbackFrameworks []byte

// TODO: add caching to avoid redundant API calls
frameworks, err := fetchFrameworks()
```

---

## Pull Requests

- One PR = one feature or fix — keep it focused
- Always target `dev`
- Fill in the PR description — summarize what changed and why
- Assign at least 1 reviewer before submitting
- The **author cannot approve their own PR**
- Once approved and all conversations resolved, anyone can merge

---

## Getting Started

```bash
git clone git@github.com:slouowzee/KAPI.git
cd KAPI
git checkout dev
git checkout -b feat/your-feature
# ... do your work ...
git commit -m "feat: add your feature" -m "explain the why, not the what"
git push origin feat/your-feature
# then open a PR on GitHub targeting dev
```

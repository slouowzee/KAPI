<p align="center">
  <img src=".github/assets/logo.svg" alt="KAPI - Keep Accelerating Project Initialization" width="600" />
</p>

Are you wondering what KAPI is? Well, if I tried to enumerate every single feature it offers, this README would weigh at least 10MB... But to keep it short: KAPI is an interactive TUI that scaffolds and configures projects based entirely on your preferences. You choose what you want, and KAPI does it for you!

<p align="center">
  <a href="https://github.com/slouowzee/kapi/releases"><img src="https://img.shields.io/github/v/release/slouowzee/kapi?style=flat-square&color=success" alt="Latest Release"></a>
  <a href="https://github.com/slouowzee/kapi/stargazers"><img src="https://img.shields.io/github/stars/slouowzee/kapi?style=flat-square&color=blue" alt="GitHub stars"></a>
  <a href="https://github.com/slouowzee/kapi/network/members"><img src="https://img.shields.io/github/forks/slouowzee/kapi?style=flat-square&color=blue" alt="GitHub forks"></a>
  <a href="https://github.com/slouowzee/kapi/issues"><img src="https://img.shields.io/github/issues/slouowzee/kapi?style=flat-square&color=blue" alt="GitHub issues"></a>
</p>

---

<p align="center">
  <img src=".github/assets/demo.gif" alt="KAPI in action" width="800" />
</p>

## Features

- **Available Environments**: Out-of-the-box support for **JavaScript/TypeScript** and **PHP**, automatically handling their specific package managers (`npm`, `bun`, `composer`, etc.).
- **Package Browser**: Real-time dependency search (npm, packagist) directly from your terminal, complete with GitHub stars and popularity metrics.
- **Git & CI Workflows**: Automated `git init`, smart `.gitignore` generation, remote repository creation, and tailored CI/CD workflows (GitHub Actions / GitLab CI).
- **Persistent Configuration**: Save your defaults and tokens via the CLI for faster scaffolding next time.

## Installation

**Homebrew (macOS / Linux)**
```bash
brew install slouowzee/kapi/kapi
```

**Scoop (Windows)**
```powershell
scoop bucket add slouowzee https://github.com/slouowzee/scoop-bucket.git
scoop install kapi
```

**AUR (Arch Linux)**
```bash
yay -S kapi-bin
```

**Or via script**
Download and install the latest binary automatically:
```bash
curl -fsSL https://raw.githubusercontent.com/slouowzee/kapi/main/install.sh | bash
```


## Usage

Run `kapi` in your terminal and follow the prompts:

```bash
kapi
```

### Keybindings
- `Up` / `Down` or `k` / `j`: Move up and down lists
- `Left` / `Right` or `h` / `l`: Toggle options on the same line
- `Space`: Select or unselect a package
- `Enter`: Confirm and move to the next step
- `Esc`: Go back a step
- `q`: Quit

### CLI Configuration

You can use KAPI commands to save your preferences for future runs:

```bash
# Add a GitHub token to access git automated features on KAPI
kapi config github.token "ghp_your_token_here"

# Set a default JS package manager
kapi config package.manager "bun"

# Read a saved config value
kapi config github.token
```


---

## Our Amazing Contributors

A massive thank you to everyone who helps make KAPI better! 

<a href="https://github.com/slouowzee/kapi/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=slouowzee/kapi" alt="KAPI Contributors" />
</a>

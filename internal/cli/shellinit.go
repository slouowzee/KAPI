package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/slouowzee/kapi/tui/styles"
)

// NOTE: these functions must stay in sync with install.sh, which embeds the
// same integration for users installing through the script.
const posixShellInit = `# kapi shell integration — do not remove
kapi() {
  KAPI_SHELL_WRAPPER=1 command kapi "$@"
  _kapi_exit=$?
  if [ -f "$HOME/.kapi_last_cd" ]; then
    _kapi_dir=$(cat "$HOME/.kapi_last_cd")
    rm -f "$HOME/.kapi_last_cd"
    [ -n "$_kapi_dir" ] && cd "$_kapi_dir"
  fi
  return $_kapi_exit
}
`

const fishShellInit = `# kapi shell integration — do not remove
function kapi
  set -lx KAPI_SHELL_WRAPPER 1
  command kapi $argv
  set _kapi_exit $status
  if test -f "$HOME/.kapi_last_cd"
    set _kapi_dir (cat "$HOME/.kapi_last_cd")
    rm -f "$HOME/.kapi_last_cd"
    test -n "$_kapi_dir"; and cd $_kapi_dir
  end
  return $_kapi_exit
end
`

const nuShellInit = `# kapi shell integration — do not remove
def --env kapi [...args: string] {
  with-env { KAPI_SHELL_WRAPPER: "1" } { run-external "kapi" ...$args }
  let cd_file = ($env.HOME | path join ".kapi_last_cd")
  if ($cd_file | path exists) {
    let dir = (open $cd_file | str trim)
    rm $cd_file
    if ($dir | is-not-empty) {
      cd $dir
    }
  }
}
`

const powershellShellInit = `# kapi shell integration — do not remove
function kapi {
  $env:KAPI_SHELL_WRAPPER = "1"
  try {
    $exe = Get-Command kapi -CommandType Application | Select-Object -First 1
    & $exe.Source @args
    $kapiExit = $LASTEXITCODE
  } finally {
    Remove-Item Env:KAPI_SHELL_WRAPPER -ErrorAction SilentlyContinue
  }
  $cdFile = Join-Path $HOME ".kapi_last_cd"
  if (Test-Path $cdFile) {
    $dir = (Get-Content $cdFile -Raw).Trim()
    Remove-Item $cdFile
    if ($dir) { Set-Location $dir }
  }
  $global:LASTEXITCODE = $kapiExit
}
`

func shellInitScript(shell string) (string, bool) {
	switch strings.ToLower(shell) {
	case "bash", "zsh", "sh", "ksh", "dash":
		return posixShellInit, true
	case "fish":
		return fishShellInit, true
	case "nu", "nushell":
		return nuShellInit, true
	case "powershell", "pwsh":
		return powershellShellInit, true
	}
	return "", false
}

func detectShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}
	if os.Getenv("PSModulePath") != "" {
		return "powershell"
	}
	return ""
}

// HandleShellInit prints the shell function that lets kapi cd into the
// project it just created.
func HandleShellInit(args []string) {
	shell := detectShell()
	if len(args) > 0 {
		shell = args[0]
	}
	if shell == "--help" || shell == "-h" {
		printShellInitHelp()
		return
	}

	script, ok := shellInitScript(shell)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unsupported shell %q. Supported shells: bash, zsh, fish, nu, powershell\n", shell)
		os.Exit(1)
	}
	fmt.Print(script)
}

func printShellInitHelp() {
	PrintLogoAndTitle("Shell integration")
	fmt.Println("  Lets kapi cd into the project it just created.")
	fmt.Println()
	fmt.Println(styles.MutedStyle.Render("  Usage:"))
	fmt.Println("    " + styles.SelectedStyle.Render("kapi shell-init [shell]") + "  Print the integration (shell detected from $SHELL)")
	fmt.Println()
	fmt.Println(styles.MutedStyle.Render("  Setup:"))
	fmt.Println("    bash / zsh    " + styles.DimStyle.Render(`add  eval "$(kapi shell-init zsh)"  to ~/.bashrc or ~/.zshrc`))
	fmt.Println("    fish          " + styles.DimStyle.Render("add  kapi shell-init fish | source  to ~/.config/fish/config.fish"))
	fmt.Println("    nushell       " + styles.DimStyle.Render("run  kapi shell-init nu | save -f ~/.config/nushell/kapi.nu  and source it in config.nu"))
	fmt.Println("    powershell    " + styles.DimStyle.Render("add  Invoke-Expression (& kapi shell-init powershell | Out-String)  to $PROFILE"))
	fmt.Println()
}

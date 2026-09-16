#!/bin/sh
set -e

REPO="slouowzee/kapi"
BINARY="kapi"
INSTALL_DIR="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin)               OS="darwin"  ;;
  linux)                OS="linux"   ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Error: could not fetch latest version from GitHub."
  exit 1
fi

if [ "$OS" = "windows" ]; then
  ASSET="${BINARY}_${OS}_${ARCH}.zip"
else
  ASSET="${BINARY}_${OS}_${ARCH}.tar.gz"
fi

DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

echo "Installing ${BINARY} ${VERSION} (${OS}/${ARCH})..."

TMP_DIR=$(mktemp -d)
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ASSET"; then
  echo "Error: download failed from ${DOWNLOAD_URL}"
  rm -rf "$TMP_DIR"
  exit 1
fi

if [ "$OS" = "windows" ]; then
  unzip -q "$TMP_DIR/$ASSET" -d "$TMP_DIR"
  TMP_BIN="$TMP_DIR/${BINARY}.exe"
  DEST="${INSTALL_DIR}/${BINARY}.exe"
else
  tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
  TMP_BIN="$TMP_DIR/${BINARY}"
  DEST="${INSTALL_DIR}/${BINARY}"
fi

chmod +x "$TMP_BIN"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_BIN" "$DEST"
else
  echo "Root permissions required to install to ${INSTALL_DIR}."
  sudo mv "$TMP_BIN" "$DEST"
fi
rm -rf "$TMP_DIR"

if [ "$OS" = "darwin" ]; then
  xattr -c "$DEST" 2>/dev/null || true
fi

echo ""
echo "kapi ${VERSION} installed to ${DEST}."

SHELL_FN='# kapi shell integration — do not remove
kapi() {
  KAPI_SHELL_WRAPPER=1 command kapi "$@"
  _kapi_exit=$?
  if [ -f "$HOME/.kapi_last_cd" ]; then
    _kapi_dir=$(cat "$HOME/.kapi_last_cd")
    rm -f "$HOME/.kapi_last_cd"
    [ -n "$_kapi_dir" ] && cd "$_kapi_dir"
  fi
  return $_kapi_exit
}'

FISH_FN='# kapi shell integration — do not remove
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
end'

NU_FN='# kapi shell integration — do not remove
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
}'

add_to_file() {
  cfg="$1"
  fn="$2"
  if grep -q "kapi shell integration" "$cfg" 2>/dev/null; then
    echo "Shell integration already present in ${cfg}. Nothing to do."
    return
  fi
  printf '\n%s\n' "$fn" >> "$cfg"
  echo "Shell integration added to ${cfg}."
  echo "Run: source ${cfg}"
}

SHELL_NAME=$(basename "${SHELL:-sh}")

printf "\nSet up shell integration for automatic cd after scaffold? [Y/n] "
# When piped (curl | bash), stdin is the script itself: read the answer from
# the terminal instead, and fall back to the default without one.
REPLY=""
if [ -t 0 ]; then
  read -r REPLY
elif { : < /dev/tty; } 2>/dev/null; then
  read -r REPLY < /dev/tty
else
  echo ""
fi

case "$REPLY" in
  [Nn]*)
    echo ""
    echo "Skipped. To enable later, add this to your shell config:"
    case "$SHELL_NAME" in
      fish)            printf '%s\n' "$FISH_FN" ;;
      nu|nushell)      printf '%s\n' "$NU_FN"   ;;
      *)               printf '%s\n' "$SHELL_FN" ;;
    esac
    ;;
  *)
    case "$SHELL_NAME" in
      zsh)
        add_to_file "$HOME/.zshrc" "$SHELL_FN"
        ;;
      bash)
        if [ "$OS" = "darwin" ]; then
          add_to_file "$HOME/.bash_profile" "$SHELL_FN"
        else
          add_to_file "$HOME/.bashrc" "$SHELL_FN"
        fi
        ;;
      ksh)
        add_to_file "$HOME/.kshrc" "$SHELL_FN"
        ;;
      dash)
        add_to_file "$HOME/.profile" "$SHELL_FN"
        ;;
      fish)
        FISH_CFG="$HOME/.config/fish/config.fish"
        mkdir -p "$(dirname "$FISH_CFG")"
        add_to_file "$FISH_CFG" "$FISH_FN"
        ;;
      nu|nushell)
        NU_CFG="$HOME/.config/nushell/config.nu"
        mkdir -p "$(dirname "$NU_CFG")"
        add_to_file "$NU_CFG" "$NU_FN"
        ;;
      *)
        echo "Unknown shell: ${SHELL_NAME}."
        echo "Add this manually to your shell config:"
        printf '%s\n' "$SHELL_FN"
        ;;
    esac
    ;;
esac

echo ""
echo "Run 'kapi' to get started."

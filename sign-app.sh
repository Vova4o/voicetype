#!/bin/sh
set -eu
sign_app=${1:?укажите путь к приложению}
if [ -n "${VOICETYPE_SIGNING_IDENTITY:-}" ]; then
 /usr/bin/codesign --force --sign "$VOICETYPE_SIGNING_IDENTITY" --identifier local.voicetype.app --timestamp=none "$sign_app"
else
 sign_dir="$HOME/.local/share/voicetype/signing"
 if [ ! -f "$sign_dir/identity" ] || [ ! -f "$sign_dir/password" ]; then
  printf '%s\n' 'Сначала выполните ./setup-signing.sh или задайте VOICETYPE_SIGNING_IDENTITY. Одноразовая подпись сбрасывает разрешения macOS.' >&2
  exit 1
 fi
 sign_keychain="$sign_dir/signing.keychain-db"
 /usr/bin/security unlock-keychain -p "$(cat "$sign_dir/password")" "$sign_keychain"
 trap '/usr/bin/security lock-keychain "$sign_keychain"' EXIT
 /usr/bin/codesign --force --sign "$(cat "$sign_dir/identity")" --keychain "$sign_keychain" \
  --identifier local.voicetype.app --timestamp=none "$sign_app"
fi
/usr/bin/codesign --verify --strict "$sign_app"

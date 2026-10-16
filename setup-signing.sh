#!/bin/sh
set -eu
umask 077
sign_dir="$HOME/.local/share/voicetype/signing"
sign_keychain="$sign_dir/signing.keychain-db"
register_signing_keychain() {
 python3 - "$sign_keychain" <<'PYKEYCHAIN'
import shlex
import subprocess
import sys
keychain = sys.argv[1]
current = shlex.split(subprocess.check_output(['/usr/bin/security', 'list-keychains', '-d', 'user'], text=True))
if keychain not in current:
    subprocess.run(['/usr/bin/security', 'list-keychains', '-d', 'user', '-s', *current, keychain], check=True)
PYKEYCHAIN
}
if [ -f "$sign_dir/identity" ] && [ -f "$sign_keychain" ]; then
 register_signing_keychain
 printf '%s\n' 'Постоянная подпись Voicetype уже настроена.'
 exit 0
fi
if [ -e "$sign_keychain" ]; then
 printf '%s\n' 'Найдена незавершённая настройка подписи; проверьте каталог ~/.local/share/voicetype/signing.' >&2
 exit 1
fi
mkdir -p "$sign_dir"
chmod 700 "$sign_dir"
sign_temp=$(mktemp -d "${TMPDIR:-/tmp}/voicetype-signing.XXXXXX")
trap 'find "$sign_temp" -type f -exec unlink {} \;; rmdir "$sign_temp"' EXIT
cat > "$sign_temp/cert.cnf" <<'CONFIG'
[req]
distinguished_name=dn
x509_extensions=ext
prompt=no
[dn]
CN=Voicetype Local Signing
[ext]
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature
extendedKeyUsage=critical,codeSigning
CONFIG
/usr/bin/openssl rand -hex 32 > "$sign_dir/password"
/usr/bin/openssl req -new -newkey rsa:2048 -nodes -x509 -days 3650 \
 -config "$sign_temp/cert.cnf" -keyout "$sign_temp/key.pem" -out "$sign_dir/certificate.pem" 2> "$sign_temp/openssl.log"
/usr/bin/openssl pkcs12 -export -inkey "$sign_temp/key.pem" -in "$sign_dir/certificate.pem" \
 -name 'Voicetype Local Signing' -out "$sign_temp/signing.p12" -passout "file:$sign_dir/password"
sign_password=$(cat "$sign_dir/password")
/usr/bin/security create-keychain -p "$sign_password" "$sign_keychain"
register_signing_keychain
/usr/bin/security unlock-keychain -p "$sign_password" "$sign_keychain"
/usr/bin/security import "$sign_temp/signing.p12" -k "$sign_keychain" -P "$sign_password" -T /usr/bin/codesign -x > /dev/null
/usr/bin/security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$sign_password" "$sign_keychain" > /dev/null
# Trust is limited to code signing; this certificate is not trusted for HTTPS.
/usr/bin/security add-trusted-cert -r trustRoot -p codeSign -k "$sign_keychain" "$sign_dir/certificate.pem"
/usr/bin/security find-identity -v -p codesigning "$sign_keychain"
/usr/bin/openssl x509 -in "$sign_dir/certificate.pem" -noout -fingerprint -sha1 | sed 's/.*=//;s/://g' > "$sign_dir/identity"
/usr/bin/security lock-keychain "$sign_keychain"
printf '%s\n' 'Постоянная локальная подпись Voicetype настроена.'

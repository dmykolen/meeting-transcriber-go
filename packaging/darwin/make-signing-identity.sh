#!/bin/bash
# Create a local code-signing identity, so macOS stops asking for the microphone
# permission after every rebuild.
#
# WHY THIS EXISTS
#
#   An ad-hoc signature (`codesign -s -`) contains a hash of the binary itself.
#   macOS ties a privacy grant to that hash, so every rebuild is, as far as TCC
#   is concerned, a different application — and it asks again. Every time.
#
#   A certificate is a stable identity. Signing with one makes the requirement
#
#       identifier "com.dmykolen.meetingtranscriber" and certificate leaf = H"<cert hash>"
#
#   which no amount of rebuilding disturbs. Apple charges $99/year for a
#   certificate they vouch for; nothing here needs anybody but this machine to
#   believe it.
#
# WHAT IT DOES
#
#   1. Generates a 10-year self-signed certificate marked for code signing.
#      The private key never leaves this machine and is not sent anywhere.
#   2. Imports it into YOUR login keychain, allowing /usr/bin/codesign to use it.
#   3. Nothing else. No sudo, no system trust, no authority beyond signing local
#      builds of this one app.
#
#   The certificate is deliberately NOT added to the trust store. codesign does
#   not need trust to sign, and the designated requirement it produces is the
#   same either way — verified, not assumed.
#
# TO UNDO
#
#   Open Keychain Access, find "Meeting Transcriber Local" under login, delete
#   it. Builds fall back to ad-hoc signing on their own.

set -euo pipefail

NAME="Meeting Transcriber Local"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"

# Identities are listed WITHOUT -v on purpose. -v means "valid", which for
# codesigning means trusted, which a self-signed certificate never is — an
# earlier version of this script checked with -v, never found what it had just
# created, and cheerfully made a second one every time it ran.
fingerprints() {
	security find-identity -p codesigning 2>/dev/null |
		awk -v name="$NAME" '$0 ~ name {print $2}'
}

found=$(fingerprints || true)
count=$(printf '%s' "$found" | grep -c . || true)

if [ "$count" -gt 0 ]; then
	echo "\"$NAME\" already exists — nothing to do."
	if [ "$count" -gt 1 ]; then
		echo
		echo "There are $count of them, which is untidy but harmless: the build signs by"
		echo "fingerprint, not by name, so it is never ambiguous. To clean up, open"
		echo "Keychain Access, search for \"$NAME\" under login, and delete all but one."
	fi
	echo
	echo "Builds use $(printf '%s' "$found" | head -1)."
	echo "Run 'make run' and grant the microphone permission once; it sticks after that."
	exit 0
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo "Creating a local code-signing certificate…"
cat > "$WORK/openssl.cnf" <<'CNF'
[req]
distinguished_name = dn
x509_extensions    = v3
prompt             = no
[dn]
CN = Meeting Transcriber Local
[v3]
basicConstraints   = critical,CA:false
keyUsage           = critical,digitalSignature
extendedKeyUsage   = critical,codeSigning
CNF

openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
	-keyout "$WORK/key.pem" -out "$WORK/cert.pem" \
	-config "$WORK/openssl.cnf" >/dev/null 2>&1

# macOS cannot read the PKCS#12 that OpenSSL 3 writes by default, so the old
# algorithms are named explicitly. The passphrase only protects the file for the
# few seconds it exists inside this script's temporary directory.
openssl pkcs12 -export -inkey "$WORK/key.pem" -in "$WORK/cert.pem" \
	-out "$WORK/identity.p12" -name "$NAME" -passout pass:mt \
	-keypbe PBE-SHA1-3DES -certpbe PBE-SHA1-3DES -macalg sha1 >/dev/null 2>&1

echo "Storing it in your login keychain — macOS may ask for your password."
security import "$WORK/identity.p12" -k "$KEYCHAIN" -P mt \
	-T /usr/bin/codesign -T /usr/bin/security >/dev/null

found=$(fingerprints || true)
if [ -z "$found" ]; then
	echo >&2
	echo "The certificate did not end up in the keychain. Builds stay ad-hoc signed," >&2
	echo "which works — macOS will just keep asking after each rebuild." >&2
	exit 1
fi

echo
echo "Done. Builds are signed by $(printf '%s' "$found" | head -1)."
echo "Run 'make run' once more and grant the microphone permission a final time;"
echo "after that it survives every rebuild."
echo
echo "The first build may put up one \"codesign wants to use your keychain\" prompt."
echo "Choose Always Allow and it will not come back."

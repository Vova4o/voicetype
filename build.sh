#!/bin/sh
set -eu
cd "$(dirname "$0")"
if [ -z "${SDKROOT:-}" ] && [ -d /Library/Developer/CommandLineTools/SDKs/MacOSX26.5.sdk ]; then
 export SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX26.5.sdk
fi
go build -o voicetype .
mkdir -p Voicetype.app/Contents/MacOS
cp voicetype Voicetype.app/Contents/MacOS/voicetype
codesign --force --sign - --identifier local.voicetype.app Voicetype.app

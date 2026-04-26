#!/bin/bash
set -e
cd "$(dirname "$0")/.."
flutter pub get
dart run build_runner build --delete-conflicting-outputs
echo "Code generation complete."

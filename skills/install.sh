#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-./skills}"

echo "Instalando RBot Max Skill Pack en: $TARGET_DIR"
mkdir -p "$TARGET_DIR"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for dir in "$SCRIPT_DIR"/*/; do
  [ -f "$dir/SKILL.md" ] || continue
  name="$(basename "$dir")"
  echo "Copiando $name"
  cp -r "$dir" "$TARGET_DIR/"
done

echo "Listo."
echo "Ahora ejecuta: ./bin/rbot skills scan"

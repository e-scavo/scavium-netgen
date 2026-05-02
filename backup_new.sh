#!/bin/bash

set -e

PROJECT_NAME="scavium-netgen"
ZIP_NAME_ROOT="scavium-netgen"

if [ -z "$1" ]; then
  echo "❌ Uso: ./backup_new.sh <etapa>"
  exit 1
fi

STAGE="$1"
ZIP_NAME="${ZIP_NAME_ROOT}-${STAGE}.zip"

echo "🧹 Eliminando zip anterior..."
rm -f "../$ZIP_NAME"
rm -f "$ZIP_NAME"

echo "📦 Generando backup..."

cd ..
echo "📂 $(pwd)"

echo "📦 Creando zip..."
zip -r "$ZIP_NAME" "$PROJECT_NAME" \
  -x "$PROJECT_NAME/.git/*" \
     "$PROJECT_NAME/node_modules/*" \
     "$PROJECT_NAME/*.log" \
     "$PROJECT_NAME/.env" \
     "$PROJECT_NAME/.DS_Store" \
     "$PROJECT_NAME/build/*" \
     "$PROJECT_NAME/cmd/scavo_geryon_be/scavo_geryon*" \
     "$PROJECT_NAME/cmd/scavo_geryon_be_v2/scavo_geryon*" \
     "$PROJECT_NAME/cmd/scavium_network_be/*" \
     "$PROJECT_NAME/cmd/scavo_mercadopago/*" \
     "$PROJECT_NAME/cmd/scavo_site_be/*" \
     "$PROJECT_NAME/cmd/scavo_site_fe/*" \
     "$PROJECT_NAME/cmd/scavo_tgbot/*" \
     "$PROJECT_NAME/cmd/scavo_wallets/*" \
     "$PROJECT_NAME/*.zip" \
     "$PROJECT_NAME/*.exe" \
     "$PROJECT_NAME/*.out"

echo "📂 Moviendo zip al proyecto..."
mv "$ZIP_NAME" "$PROJECT_NAME/"

cd "$PROJECT_NAME"

echo "✅ Backup generado: $ZIP_NAME"
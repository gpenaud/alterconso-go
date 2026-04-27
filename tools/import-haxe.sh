#!/usr/bin/env bash
# ============================================================
# import-haxe.sh
# Convertit un dump SQL Alterconso (Haxe, PascalCase) en un dump
# d'import compatible avec le code Go (GORM, snake_case),
# en purgeant les tables transitoires inutiles.
#
# Usage :
#   ./tools/import-haxe.sh <last-backup.sql> [output.sql]
#   # défaut : output = import-go.sql à la racine du projet
#
# Variables d'environnement (avec valeurs par défaut) :
#   DB_HOST     localhost
#   DB_PORT     3306
#   DB_USER     alterconso
#   DB_PASSWORD changeme
#   TMP_DB      alterconso_haxe_import   (base temporaire)
#
# Le script :
#   1. Filtre le dump (supprime Error, Session, BufferedMail, Cache).
#   2. Drop / recrée la base temporaire.
#   3. Importe le dump filtré (tables Haxe).
#   4. Lance `./alterconso migrate` (AutoMigrate GORM) pour créer
#      les tables snake_case vides à côté.
#   5. Applique migrate-haxe-to-gorm.sql (copie + transformation).
#   6. Supprime les tables Haxe et autres GORM pas utiles.
#   7. mysqldump des tables GORM seulement → output.
#   8. Drop la base temporaire.
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INPUT="${1:-}"
OUTPUT="${2:-$SCRIPT_DIR/import-go.sql}"

if [[ -z "$INPUT" || ! -f "$INPUT" ]]; then
  echo "Usage: $0 <last-backup.sql> [output.sql]"
  exit 1
fi

DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-alterconso}"
DB_PASSWORD="${DB_PASSWORD:-changeme}"
DB_ROOT_USER="${DB_ROOT_USER:-root}"
DB_ROOT_PASSWORD="${DB_ROOT_PASSWORD:-root}"
TMP_DB="${TMP_DB:-alterconso_haxe_import}"

MYSQL_OPTS=(-u "$DB_USER" -p"$DB_PASSWORD" -h "$DB_HOST" -P "$DB_PORT")
MYSQL_ROOT_OPTS=(-u "$DB_ROOT_USER" -p"$DB_ROOT_PASSWORD" -h "$DB_HOST" -P "$DB_PORT")
MYSQLDUMP_OPTS=(-u "$DB_USER" -p"$DB_PASSWORD" -h "$DB_HOST" -P "$DB_PORT")

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT
FILTERED_SQL="$WORK_DIR/filtered.sql"

log() { echo "[import-haxe] $*"; }

# ============================================================
# Étape 1 : filtrage du dump
# ============================================================
log "1/8 — filtrage des tables transitoires (Error, Session, BufferedMail, Cache)..."
awk '
BEGIN { skip = 0 }
/^-- (Table structure|Dumping data) for table `[^`]+`/ {
  match($0, /`[^`]+`/)
  tbl = substr($0, RSTART+1, RLENGTH-2)
  if (tbl == "Error" || tbl == "Session" || tbl == "BufferedMail" || tbl == "Cache") {
    skip = 1
  } else {
    skip = 0
  }
}
!skip
' "$INPUT" > "$FILTERED_SQL"

ORIG_SIZE=$(stat -c%s "$INPUT")
FILT_SIZE=$(stat -c%s "$FILTERED_SQL")
log "    $(numfmt --to=iec-i --suffix=B $ORIG_SIZE) -> $(numfmt --to=iec-i --suffix=B $FILT_SIZE)"

# ============================================================
# Étape 2 : recréation de la base temporaire
# ============================================================
log "2/8 — drop/recréation de la base $TMP_DB (via root)..."
mysql "${MYSQL_ROOT_OPTS[@]}" -e "
  DROP DATABASE IF EXISTS \`$TMP_DB\`;
  CREATE DATABASE \`$TMP_DB\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
  GRANT ALL PRIVILEGES ON \`$TMP_DB\`.* TO '$DB_USER'@'%';
  FLUSH PRIVILEGES;
" 2>&1 | grep -v "Warning" || true

# ============================================================
# Étape 3 : import du dump filtré
# ============================================================
log "3/8 — import du dump Haxe filtré..."
mysql "${MYSQL_OPTS[@]}" "$TMP_DB" < "$FILTERED_SQL" 2>&1 | grep -v "Warning" || true

# ============================================================
# Étape 4 : AutoMigrate GORM (création des tables snake_case)
# ============================================================
log "4/8 — création des tables GORM via 'alterconso migrate'..."

cd "$SCRIPT_DIR"
log "    compilation du binaire..."
go build -o "$WORK_DIR/alterconso" ./cmd/server

log "    exécution AutoMigrate sur $TMP_DB..."
DB_HOST="$DB_HOST" \
DB_PORT="$DB_PORT" \
DB_USER="$DB_USER" \
DB_PASSWORD="$DB_PASSWORD" \
DB_NAME="$TMP_DB" \
"$WORK_DIR/alterconso" migrate

# ============================================================
# Étape 5 : application de la transformation Haxe -> GORM
# ============================================================
log "5/8 — application de migrate-haxe-to-gorm.sql..."
mysql "${MYSQL_OPTS[@]}" "$TMP_DB" < "$SCRIPT_DIR/migrate-haxe-to-gorm.sql" 2>&1 | grep -v "Warning" | tail -20 || true

# ============================================================
# Étape 6 : suppression des tables Haxe résiduelles
# ============================================================
log "6/8 — suppression des tables Haxe (PascalCase) sauf File..."
# File a le même nom (PascalCase) côté GORM — on le garde.
HAXE_TABLES=$(mysql "${MYSQL_OPTS[@]}" -N -e "
  SELECT GROUP_CONCAT(TABLE_NAME SEPARATOR '\`,\`')
  FROM information_schema.TABLES
  WHERE TABLE_SCHEMA = '$TMP_DB'
    AND TABLE_NAME NOT REGEXP '^[a-z]'
    AND TABLE_NAME <> 'File'
" "$TMP_DB" 2>&1 | grep -v "Warning" | tail -1)

if [[ -n "$HAXE_TABLES" && "$HAXE_TABLES" != "NULL" ]]; then
  mysql "${MYSQL_OPTS[@]}" "$TMP_DB" -e "
    SET FOREIGN_KEY_CHECKS=0;
    DROP TABLE IF EXISTS \`$HAXE_TABLES\`;
    SET FOREIGN_KEY_CHECKS=1;
  " 2>&1 | grep -v "Warning" || true
fi

# ============================================================
# Étape 7 : mysqldump propre
# ============================================================
log "7/8 — mysqldump vers $OUTPUT..."
mysqldump "${MYSQLDUMP_OPTS[@]}" \
  --no-tablespaces \
  --skip-comments \
  --single-transaction \
  --default-character-set=utf8mb4 \
  --hex-blob \
  "$TMP_DB" > "$OUTPUT" 2>/dev/null

OUT_SIZE=$(stat -c%s "$OUTPUT")
log "    dump généré : $(numfmt --to=iec-i --suffix=B $OUT_SIZE)"

# ============================================================
# Étape 8 : nettoyage
# ============================================================
log "8/8 — drop de la base temporaire $TMP_DB..."
mysql "${MYSQL_ROOT_OPTS[@]}" -e "DROP DATABASE \`$TMP_DB\`;" 2>&1 | grep -v "Warning" || true

# Récap
log ""
log "✓ terminé. Pour importer dans une nouvelle base alterconso :"
log "    mysql -u $DB_USER -p$DB_PASSWORD alterconso < $OUTPUT"

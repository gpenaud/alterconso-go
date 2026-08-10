// Command rehash : enrobe en bcrypt (schéma bm:) tous les mots de passe
// encore en MD5 legacy nu, SANS mot de passe en clair. Idempotent et
// ré-exécutable (à relancer après chaque ré-import de la base legacy).
//
// Conservé pour exécution locale (`go run ./cmd/rehash`). En production
// distroless, utiliser la sous-commande du binaire serveur :
// `/app/alterconso rehash` (cf. cmd/server).
//
// Usage :
//
//	go run ./cmd/rehash             # applique
//	go run ./cmd/rehash -dry-run    # compte seulement
package main

import (
	"flag"
	"log"
	"os"

	"github.com/gpenaud/alterconso/internal/config"
	"github.com/gpenaud/alterconso/internal/db"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "ne rien écrire, juste compter les lignes legacy")
	batchSize := flag.Int("batch", 200, "taille des lots")
	flag.Parse()

	// Le batch n'envoie aucune notification : neutralise la validation du bloc
	// notifications sans rapport avec le rehash (cf. cmd/server runRehash).
	_ = os.Setenv("NOTIFICATIONS_ENABLED", "false")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	database, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	if _, _, _, err := db.RehashLegacyPasswords(database, *dryRun, *batchSize); err != nil {
		log.Fatalf("rehash: %v", err)
	}
}

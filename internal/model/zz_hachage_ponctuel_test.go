package model

import (
	"fmt"
	"testing"
)

// Utilitaire ponctuel : produit le hachage du schéma cible avec le code du
// projet, plutôt qu'en le réimplémentant à côté. Ce fichier est supprimé
// aussitôt après usage.
func TestGenererHachagePonctuel(t *testing.T) {
	h := HashPassword("azerty")
	ok, _ := VerifyPassword(h, "azerty", "")
	if !ok {
		t.Fatal("le hachage produit ne se vérifie pas")
	}
	fmt.Println("HACHAGE=" + h)
}

package handler

import (
	"os"
	"path/filepath"
	"testing"
)

// L'empreinte doit se calculer sur les feuilles TELLES QUE L'IMAGE LES PORTE :
// precompressees, sans le fichier source. Sans cela elle retombait sur « dev »
// en production — figee, donc sans effet sur le cache qu'elle devait dejouer.
func TestEmpreinteLitLesFeuillesCompressees(t *testing.T) {
	dir := t.TempDir()
	css := filepath.Join(dir, "alterconso.css")

	// Le cas du developpement : le fichier brut.
	if err := os.WriteFile(css, []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	brut := empreinteDes([]string{css})
	if brut == "dev" {
		t.Fatal("le fichier brut n'a pas ete lu")
	}

	// Le cas de l'image : seule la forme compressee existe.
	if err := os.Remove(css); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(css+".gz", []byte("gzip-ish"), 0o644); err != nil {
		t.Fatal(err)
	}
	compresse := empreinteDes([]string{css})
	if compresse == "dev" {
		t.Error("la forme .gz n'a pas ete lue : l'empreinte reste figee en production")
	}
	if compresse == brut {
		t.Error("deux contenus differents rendent la meme empreinte")
	}

	// Rien de lisible : on ne casse pas la page pour autant.
	if err := os.Remove(css + ".gz"); err != nil {
		t.Fatal(err)
	}
	if empreinteDes([]string{css}) != "dev" {
		t.Error("sans aucun fichier, l'empreinte devrait valoir « dev »")
	}
}

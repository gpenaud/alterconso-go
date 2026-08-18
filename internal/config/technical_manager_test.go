package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Les configurations deployees portent encore la cle `superadmin`. Ignoree,
// elle laisserait l'installation sans aucun responsable technique — et le
// compte concerne ne serait plus reconnu que par ses droits en base.
func TestLegacySuperAdminKeyStillDesignatesTechnicalManager(t *testing.T) {
	cfg := writeAndLoad(t, `
db_password: x
jwt_secret: y
key: z
notifications:
  enabled: false
superadmin:
  email: alterconso@leportail.org
  first_name: Super
  last_name: Admin
`)

	if got := cfg.TechnicalManager.Email; got != "alterconso@leportail.org" {
		t.Fatalf("adresse heritee attendue, obtenu %q", got)
	}
	if !cfg.IsTechnicalManager("Alterconso@LePortail.org") {
		t.Error("le role doit etre reconnu, casse comprise")
	}
}

// Le nom courant prime quand les deux cles coexistent.
func TestCurrentKeyWinsOverLegacy(t *testing.T) {
	cfg := writeAndLoad(t, `
db_password: x
jwt_secret: y
key: z
notifications:
  enabled: false
technical_manager:
  email: technique@exemple.fr
superadmin:
  email: ancien@exemple.fr
  password: motdepasse
`)

	if got := cfg.TechnicalManager.Email; got != "technique@exemple.fr" {
		t.Errorf("adresse courante attendue, obtenu %q", got)
	}
	// Les champs que le nom courant ne renseigne pas se completent depuis
	// l'ancien : une configuration de transition peut n'en porter qu'une part.
	if got := cfg.TechnicalManager.Password; got != "motdepasse" {
		t.Errorf("mot de passe herite attendu, obtenu %q", got)
	}
}

// Sans aucune des deux cles, personne ne tient le role.
func TestNoTechnicalManagerConfigured(t *testing.T) {
	cfg := writeAndLoad(t, "db_password: x\njwt_secret: y\nkey: z\nnotifications:\n  enabled: false\n")

	if cfg.IsTechnicalManager("qui@exemple.fr") {
		t.Error("aucune adresse configuree : personne ne tient le role")
	}
	if cfg.IsTechnicalManager("") {
		t.Error("une adresse vide ne tient pas le role")
	}
}

func writeAndLoad(t *testing.T, yaml string) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("ecriture : %v", err)
	}
	t.Setenv("CONFIG_FILE", path)
	// Les variables d'environnement de la machine de test ne doivent pas
	// s'inviter dans le resultat.
	for _, k := range []string{"SUPERADMIN_EMAIL", "SUPERADMIN_PASSWORD",
		"TECHNICAL_MANAGER_EMAIL", "TECHNICAL_MANAGER_PASSWORD"} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("chargement : %v", err)
	}
	return cfg
}

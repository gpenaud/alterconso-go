package model

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Schémas de stockage du mot de passe (colonne users.pass).
//
// Trois formats coexistent volontairement et durablement, pour supporter
// les ré-imports de la base legacy sans rien casser :
//
//   - legacy : 32 hex minuscules = md5(appKey + password). Écrit par l'import
//     legacy et par les anciennes versions. Toujours vérifiable.
//   - bm:    : bcrypt(md5(appKey + password)). Produit par le batch cmd/rehash
//     sans mot de passe en clair. Durcit la base sans login utilisateur.
//   - b2:    : bcrypt(sha256hex(password)). Schéma cible, aucune dépendance au
//     pepper applicatif. Produit par SetPassword et par le re-hash au login.
//
// Le sha256 interne du schéma b2 supprime la troncature bcrypt à 72 octets
// (entrée toujours = 64 caractères hex) et rend SetPassword sans erreur en
// pratique quelle que soit la longueur du mot de passe saisi.
const (
	schemeBcryptModern = "b2:" // bcrypt(sha256hex(plain))
	schemeBcryptLegacy = "bm:" // bcrypt(md5(appKey+plain))
)

var legacyMD5Re = regexp.MustCompile(`^[0-9a-f]{32}$`)

func md5Hex(appKey, plain string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(appKey+strings.TrimSpace(plain))))
}

func sha256Hex(plain string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(plain))))
}

// HashPassword produit le schéma cible (b2:) pour un nouveau mot de passe.
func HashPassword(plain string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(sha256Hex(plain)), bcrypt.DefaultCost)
	if err != nil {
		// Inatteignable en pratique : entrée fixe de 64 octets, coût valide.
		panic(fmt.Errorf("bcrypt: %w", err))
	}
	return schemeBcryptModern + string(h)
}

// WrapLegacyMD5 enrobe un hash MD5 legacy existant en bcrypt SANS le mot de
// passe en clair. Utilisé par le batch cmd/rehash. Idempotent côté appelant :
// ne doit être appelé que sur une valeur reconnue par IsLegacyMD5.
func WrapLegacyMD5(legacyHex string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(legacyHex), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return schemeBcryptLegacy + string(h), nil
}

// IsLegacyMD5 indique si la valeur stockée est un MD5 legacy nu (donc à
// enrober par le batch, ou fraîchement ré-importé depuis le legacy).
func IsLegacyMD5(stored string) bool {
	return legacyMD5Re.MatchString(stored)
}

// VerifyPassword vérifie un mot de passe contre la valeur stockée, quel que
// soit son schéma. needsUpgrade=true signale que la connexion réussie devrait
// déclencher une réécriture vers le schéma cible (b2:) — on détient alors le
// mot de passe en clair.
func VerifyPassword(stored, plain, appKey string) (ok, needsUpgrade bool) {
	switch {
	case stored == "":
		return false, false

	case strings.HasPrefix(stored, schemeBcryptModern):
		err := bcrypt.CompareHashAndPassword(
			[]byte(strings.TrimPrefix(stored, schemeBcryptModern)),
			[]byte(sha256Hex(plain)))
		return err == nil, false

	case strings.HasPrefix(stored, schemeBcryptLegacy):
		err := bcrypt.CompareHashAndPassword(
			[]byte(strings.TrimPrefix(stored, schemeBcryptLegacy)),
			[]byte(md5Hex(appKey, plain)))
		// Valide mais dépend encore du pepper interne : à migrer vers b2.
		return err == nil, err == nil

	case IsLegacyMD5(stored):
		match := subtle.ConstantTimeCompare(
			[]byte(stored), []byte(md5Hex(appKey, plain))) == 1
		return match, match

	default:
		// Filet de sécurité : bcrypt nu éventuel ($2...) écrit par une
		// version antérieure = bcrypt(plain). Migré au prochain login.
		if strings.HasPrefix(stored, "$2") {
			err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(strings.TrimSpace(plain)))
			return err == nil, err == nil
		}
		return false, false
	}
}

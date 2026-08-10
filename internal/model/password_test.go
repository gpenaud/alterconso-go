package model

import (
	"crypto/md5"
	"fmt"
	"testing"
)

const testPepper = "localdevkey"

func legacyMD5(pepper, plain string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(pepper+plain)))
}

// Vérifie les trois schémas + le drapeau needsUpgrade, et le cas ré-import.
func TestVerifyPasswordAcrossSchemes(t *testing.T) {
	const pwd = "s3cr3t-pa$$"

	// 1) MD5 legacy nu (tel qu'importé depuis le legacy) : valide, à migrer.
	legacy := legacyMD5(testPepper, pwd)
	if !IsLegacyMD5(legacy) {
		t.Fatalf("hash legacy non reconnu: %s", legacy)
	}
	if ok, up := VerifyPassword(legacy, pwd, testPepper); !ok || !up {
		t.Fatalf("legacy: ok=%v up=%v, attendu true,true", ok, up)
	}
	if ok, _ := VerifyPassword(legacy, "mauvais", testPepper); ok {
		t.Fatal("legacy: mauvais mot de passe accepté")
	}

	// 2) Enrobage batch (sans clair) -> bm: ; valide, encore à migrer.
	wrapped, err := WrapLegacyMD5(legacy)
	if err != nil {
		t.Fatalf("WrapLegacyMD5: %v", err)
	}
	if IsLegacyMD5(wrapped) {
		t.Fatal("bm: ne doit pas être considéré comme legacy (idempotence batch)")
	}
	if ok, up := VerifyPassword(wrapped, pwd, testPepper); !ok || !up {
		t.Fatalf("bm: ok=%v up=%v, attendu true,true", ok, up)
	}
	if ok, _ := VerifyPassword(wrapped, "mauvais", testPepper); ok {
		t.Fatal("bm: mauvais mot de passe accepté")
	}

	// 3) Schéma cible b2: (SetPassword / re-hash login) : valide, plus de migration.
	modern := HashPassword(pwd)
	if IsLegacyMD5(modern) {
		t.Fatal("b2: ne doit pas être considéré comme legacy")
	}
	if ok, up := VerifyPassword(modern, pwd, testPepper); !ok || up {
		t.Fatalf("b2: ok=%v up=%v, attendu true,false", ok, up)
	}
	// b2 ne dépend plus du pepper : un pepper différent fonctionne toujours.
	if ok, _ := VerifyPassword(modern, pwd, "autre-cle"); !ok {
		t.Fatal("b2: doit être indépendant du pepper applicatif")
	}
	if ok, _ := VerifyPassword(modern, "mauvais", testPepper); ok {
		t.Fatal("b2: mauvais mot de passe accepté")
	}
}

// Un User migré conserve l'authentification après réécriture du hash.
func TestUserSetCheckRoundTrip(t *testing.T) {
	u := &User{}
	u.Pass = legacyMD5(testPepper, "hunter2") // état post-import

	ok, needs := u.CheckPassword("hunter2", testPepper)
	if !ok || !needs {
		t.Fatalf("post-import: ok=%v needs=%v", ok, needs)
	}

	u.SetPassword("hunter2", "") // re-hash au login
	ok, needs = u.CheckPassword("hunter2", testPepper)
	if !ok || needs {
		t.Fatalf("post-rehash: ok=%v needs=%v, attendu true,false", ok, needs)
	}
}

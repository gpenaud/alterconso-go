package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// runRightsRevoke retire un droit a un compte, groupe par groupe.
//
//	alterconso rights-revoke <email> <Droit> [--apply]
//
// Sans --apply, la commande montre ce qu'elle ferait et n'ecrit rien. C'est le
// defaut a dessein : une correction de droits passee trop vite a deja elargi
// 88 appartenances au lieu de 4, faute d'avoir regarde l'assiette avant.
func runRightsRevoke(gdb *gorm.DB, args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage : alterconso rights-revoke <email> <Droit> [--apply]")
		os.Exit(2)
	}
	email, droit := args[0], model.Right(args[1])
	apply := len(args) > 2 && args[2] == "--apply"

	var user model.User
	if err := gdb.Where("email = ?", email).First(&user).Error; err != nil {
		fmt.Fprintf(os.Stderr, "compte %q introuvable\n", email)
		os.Exit(1)
	}

	var ugs []model.UserGroup
	if err := gdb.Preload("Group").Where("user_id = ?", user.ID).Find(&ugs).Error; err != nil {
		fmt.Fprintf(os.Stderr, "lecture impossible : %v\n", err)
		os.Exit(1)
	}

	touched := 0
	for _, ug := range ugs {
		next, changed := removeRight(ug.Rights, droit)
		if !changed {
			continue
		}
		touched++
		fmt.Printf("groupe %q (%d)\n  avant : %s\n  apres : %s\n",
			ug.Group.Name, ug.GroupID, ug.Rights, next)

		// Un groupe ne doit pas se retrouver sans responsable : le droit est
		// exclusif, mais rien n'empeche deux porteurs d'y coexister, et c'est
		// justement le cas qu'on vient corriger.
		if droit == model.RightGroupAdmin && !hasAnotherGroupHead(gdb, ug.GroupID, user.ID) {
			fmt.Fprintf(os.Stderr,
				"  REFUS : personne d'autre ne porte %s sur ce groupe\n", droit)
			os.Exit(1)
		}

		if !apply {
			fmt.Println("  (essai a blanc — relancer avec --apply pour ecrire)")
			continue
		}
		if err := gdb.Model(&model.UserGroup{}).
			Where("user_id = ? AND group_id = ?", ug.UserID, ug.GroupID).
			Update("rights", next).Error; err != nil {
			fmt.Fprintf(os.Stderr, "  ecriture impossible : %v\n", err)
			os.Exit(1)
		}
		fmt.Println("  ecrit.")
	}

	if touched == 0 {
		fmt.Printf("%s ne porte %s sur aucun groupe — rien a faire.\n", email, droit)
	}
}

// hasAnotherGroupHead dit si un autre membre du groupe porte le role de
// responsable. Le responsable technique ne compte pas : son role vient de la
// configuration et ne remplace pas un responsable de groupe.
func hasAnotherGroupHead(gdb *gorm.DB, groupID, exceptUserID uint) bool {
	var ugs []model.UserGroup
	if err := gdb.Preload("User").Where("group_id = ?", groupID).Find(&ugs).Error; err != nil {
		return false
	}
	for _, ug := range ugs {
		if ug.UserID == exceptUserID {
			continue
		}
		if ug.HasRight(model.RightGroupAdmin) {
			return true
		}
	}
	return false
}

// removeRight retire toutes les occurrences d'un droit d'une liste serialisee.
func removeRight(raw string, droit model.Right) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return raw, false
	}
	var rights []model.UserRight
	if err := json.Unmarshal([]byte(raw), &rights); err != nil {
		return raw, false
	}

	out := make([]model.UserRight, 0, len(rights))
	for _, r := range rights {
		if r.Right == droit {
			continue
		}
		out = append(out, r)
	}
	if len(out) == len(rights) {
		return raw, false
	}

	b, err := json.Marshal(out)
	if err != nil {
		return raw, false
	}
	return string(b), true
}

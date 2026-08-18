package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// runRightsReport imprime les droits en vigueur, appartenance par appartenance.
//
// Une correction de donnees se verifie mal depuis les logs : ils disent ce
// qu'on a change, jamais ce qui reste. Cette commande lit et n'ecrit rien, et
// s'execute dans le conteneur — c'est le seul endroit ou la base est joignable
// sans sortir de mot de passe de son secret.
//
//	alterconso rights-report
func runRightsReport(gdb *gorm.DB) {
	var ugs []model.UserGroup
	if err := gdb.Preload("User").Preload("Group").Find(&ugs).Error; err != nil {
		fmt.Fprintf(os.Stderr, "lecture impossible : %v\n", err)
		os.Exit(1)
	}

	type ligne struct {
		email  string
		groupe string
		droits string
	}
	var lignes []ligne
	for _, ug := range ugs {
		// Les appartenances sans droit sont la majorite : les lister noierait
		// le rapport, et c'est justement ce qu'on veut voir vide.
		if len(ug.GetRights()) == 0 {
			continue
		}
		labels := ""
		for i, r := range ug.GetRights() {
			if i > 0 {
				labels += ", "
			}
			labels += string(r.Right)
			if len(r.Params) > 0 {
				labels += fmt.Sprintf("(%v)", r.Params)
			}
		}
		lignes = append(lignes, ligne{ug.User.Email, ug.Group.Name, labels})
	}

	sort.Slice(lignes, func(i, j int) bool { return lignes[i].email < lignes[j].email })
	for _, l := range lignes {
		fmt.Printf("%-40s %-28s %s\n", l.email, l.groupe, l.droits)
	}
	fmt.Printf("\n%d appartenance(s) portant au moins un droit\n", len(lignes))
}

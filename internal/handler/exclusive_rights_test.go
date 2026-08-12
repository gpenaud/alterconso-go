package handler

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Les rôles à titulaire unique se transfèrent : les accorder les retire à qui
// les détenait. Éprouvé sur une vraie base, le transfert reposant sur des
// écritures SQL que rien ne simule fidèlement.
//
// Sans TEST_DSN, le test se saute — inutile d'imposer une base à qui lance
// simplement `go test ./...`.
func TestExclusiveRightsTransfer(t *testing.T) {
	db := testDB(t)

	groupID := seedGroup(t, db)
	ancien := seedMember(t, db, groupID, "zz-ancien", `[{"right":"GroupAdmin"},{"right":"Messages"}]`)
	nouveau := seedMember(t, db, groupID, "zz-nouveau", `[]`)

	// Le nouveau reçoit le rôle : l'ancien doit le perdre, mais garder le reste.
	granted := []model.UserRight{{Right: model.RightGroupAdmin}}
	transfers, err := transferExclusiveRights(db, groupID, nouveau, granted)
	if err != nil {
		t.Fatalf("transfert : %v", err)
	}
	if _, ok := transfers[model.RightGroupAdmin]; !ok {
		t.Error("le transfert n'a pas été signalé, la dépossession serait muette")
	}

	var ug model.UserGroup
	db.Where("user_id = ? AND group_id = ?", ancien, groupID).First(&ug)
	if ug.HasRight(model.RightGroupAdmin) {
		t.Error("l'ancien responsable a gardé le rôle : il n'est donc pas exclusif")
	}
	if !ug.HasRight(model.RightMessages) {
		t.Error("le transfert a emporté les autres droits de l'ancien responsable")
	}
}

// Un droit non exclusif se cumule : l'accorder ne le retire à personne.
func TestNonExclusiveRightIsShared(t *testing.T) {
	db := testDB(t)

	groupID := seedGroup(t, db)
	premier := seedMember(t, db, groupID, "zz-premier", `[{"right":"Messages"}]`)
	second := seedMember(t, db, groupID, "zz-second", `[]`)

	granted := []model.UserRight{{Right: model.RightMessages}}
	if _, err := transferExclusiveRights(db, groupID, second, granted); err != nil {
		t.Fatalf("transfert : %v", err)
	}

	var ug model.UserGroup
	db.Where("user_id = ? AND group_id = ?", premier, groupID).First(&ug)
	if !ug.HasRight(model.RightMessages) {
		t.Error("un droit cumulable a été retiré comme s'il était exclusif")
	}
}

// Le seul responsable ne peut pas se retirer le rôle : le groupe n'aurait plus
// personne pour l'administrer.
func TestLeavesGroupWithoutManager(t *testing.T) {
	db := testDB(t)

	groupID := seedGroup(t, db)
	seul := seedMember(t, db, groupID, "zz-seul", `[{"right":"GroupAdmin"}]`)
	autre := seedMember(t, db, groupID, "zz-autre", `[{"right":"Messages"}]`)

	if !leavesGroupWithoutManager(db, groupID, seul, []model.UserRight{{Right: model.RightMessages}}) {
		t.Error("le retrait du seul responsable aurait dû être refusé")
	}
	if leavesGroupWithoutManager(db, groupID, seul, []model.UserRight{{Right: model.RightGroupAdmin}}) {
		t.Error("garder le rôle ne laisse pas le groupe sans responsable")
	}
	// Quelqu'un d'autre le détient déjà : le retrait est alors sans danger.
	db.Model(&model.UserGroup{}).Where("user_id = ? AND group_id = ?", autre, groupID).
		Update("rights", `[{"right":"GroupAdmin"}]`)
	if leavesGroupWithoutManager(db, groupID, seul, nil) {
		t.Error("un second responsable existe, le retrait devait être permis")
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN absent")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	return db
}

// seedGroup crée un groupe jetable, supprimé en fin de test.
func seedGroup(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	g := model.Group{Name: "ZZ Groupe de test"}
	if err := db.Create(&g).Error; err != nil {
		t.Fatalf("création du groupe : %v", err)
	}
	t.Cleanup(func() {
		db.Where("group_id = ?", g.ID).Delete(&model.UserGroup{})
		db.Unscoped().Delete(&model.Group{}, g.ID)
	})
	return g.ID
}

// seedMember crée un membre jetable du groupe, avec les droits donnés.
func seedMember(t *testing.T, db *gorm.DB, groupID uint, login, rights string) uint {
	t.Helper()
	u := model.User{FirstName: "Test", LastName: login, Email: login + "@example.invalid", Pass: "x"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("création de %s : %v", login, err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&model.User{}, u.ID) })

	if err := db.Create(&model.UserGroup{UserID: u.ID, GroupID: groupID, Rights: rights}).Error; err != nil {
		t.Fatalf("rattachement de %s : %v", login, err)
	}
	var check []model.UserRight
	if err := json.Unmarshal([]byte(rights), &check); err != nil {
		t.Fatalf("droits de départ illisibles : %v", err)
	}
	return u.ID
}

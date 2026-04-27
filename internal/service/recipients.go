package service

import (
	"time"

	"github.com/gpenaud/alterconso/internal/config"
	"gorm.io/gorm"
)

// UserCategoryScores retourne le score par membre du groupe pour la catégorie
// donnée. Le score se passe ensuite à cat.Match.
//
// Sémantique selon le mode :
//   - mode total : score = nombre total de commandes (distributions distinctes)
//     sur la fenêtre glissante de Tm * 30 jours.
//   - mode per-month : score = nombre de mois calendaires (sur les T derniers,
//     mois courant inclus) où le user satisfait la règle par-mois.
func UserCategoryScores(db *gorm.DB, groupID uint, memberIDs []uint, now time.Time, cat config.RecipientCategory) map[uint]int {
	if cat.PerMonth {
		return userMonthScores(db, groupID, memberIDs, now, cat)
	}
	return userTotalCounts(db, groupID, now.Add(-cat.Window))
}

// FindCategoryByName retourne la catégorie portant ce nom dans la liste,
// ou nil si non trouvée.
func FindCategoryByName(cats []config.RecipientCategory, name string) *config.RecipientCategory {
	for i := range cats {
		if cats[i].Name == name {
			return &cats[i]
		}
	}
	return nil
}

// EligibleUsersForCategory retourne les IDs des membres du groupe dont le
// score satisfait la règle de la catégorie. Pure correspondance par pattern,
// sans logique d'exclusivité mutuelle (contrairement aux buckets de la page
// /messages). Utilisé pour filtrer les destinataires des notifications cron.
func EligibleUsersForCategory(db *gorm.DB, groupID uint, now time.Time, cat config.RecipientCategory) []uint {
	var memberIDs []uint
	db.Table("user_groups").Where("group_id = ?", groupID).Pluck("user_id", &memberIDs)
	if len(memberIDs) == 0 {
		return nil
	}
	scores := UserCategoryScores(db, groupID, memberIDs, now, cat)
	out := make([]uint, 0, len(memberIDs))
	for _, uid := range memberIDs {
		if cat.Match(scores[uid]) {
			out = append(out, uid)
		}
	}
	return out
}

// userTotalCounts : map user_id → nombre total de commandes depuis `since`.
func userTotalCounts(db *gorm.DB, groupID uint, since time.Time) map[uint]int {
	type row struct {
		UserID uint
		N      int
	}
	var rows []row
	db.Raw(`
		SELECT uo.user_id AS user_id, COUNT(DISTINCT uo.distribution_id) AS n
		FROM user_orders uo
		JOIN distributions d ON d.id = uo.distribution_id
		JOIN multi_distribs md ON md.id = d.multi_distrib_id
		WHERE md.group_id = ? AND md.distrib_start_date >= ?
		GROUP BY uo.user_id
	`, groupID, since).Scan(&rows)
	out := make(map[uint]int, len(rows))
	for _, r := range rows {
		out[r.UserID] = r.N
	}
	return out
}

// userMonthScores : map user_id → nombre de mois qualifiants sur les T mois
// calendaires (mois courant inclus) à partir d'aujourd'hui.
func userMonthScores(db *gorm.DB, groupID uint, memberIDs []uint, now time.Time, cat config.RecipientCategory) map[uint]int {
	windowMonths := cat.WindowMonths()
	if windowMonths <= 0 {
		return map[uint]int{}
	}

	nowMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	since := nowMonthStart.AddDate(0, -(windowMonths - 1), 0)
	months := make([]int, windowMonths)
	for i := 0; i < windowMonths; i++ {
		m := since.AddDate(0, i, 0)
		months[i] = m.Year()*12 + int(m.Month())
	}

	type row struct {
		UserID uint
		YM     int
		Cnt    int
	}
	var rows []row
	db.Raw(`
		SELECT uo.user_id AS user_id,
		       YEAR(md.distrib_start_date)*12 + MONTH(md.distrib_start_date) AS ym,
		       COUNT(DISTINCT uo.distribution_id) AS cnt
		FROM user_orders uo
		JOIN distributions d ON d.id = uo.distribution_id
		JOIN multi_distribs md ON md.id = d.multi_distrib_id
		WHERE md.group_id = ? AND md.distrib_start_date >= ?
		GROUP BY uo.user_id, ym
	`, groupID, since).Scan(&rows)

	perUser := make(map[uint]map[int]int)
	for _, r := range rows {
		m, ok := perUser[r.UserID]
		if !ok {
			m = make(map[int]int)
			perUser[r.UserID] = m
		}
		m[r.YM] = r.Cnt
	}

	scores := make(map[uint]int, len(memberIDs))
	for _, uid := range memberIDs {
		monthMap := perUser[uid]
		n := 0
		for _, ym := range months {
			cnt := 0
			if monthMap != nil {
				cnt = monthMap[ym]
			}
			if cat.MatchPerMonthCount(cnt) {
				n++
			}
		}
		scores[uid] = n
	}
	return scores
}

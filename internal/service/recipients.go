package service

import (
	"time"

	"github.com/gpenaud/alterconso/internal/config"
	"gorm.io/gorm"
)

// UserCategoryScores retourne, pour chaque membre, un slice de scores — un
// par condition de la catégorie, dans le même ordre que cat.Conditions.
//
// Le score se passe ensuite à cat.Match(scores[uid]).
func UserCategoryScores(db *gorm.DB, groupID uint, memberIDs []uint, now time.Time, cat config.RecipientCategory) map[uint][]int {
	out := make(map[uint][]int, len(memberIDs))
	for _, uid := range memberIDs {
		out[uid] = make([]int, len(cat.Conditions))
	}

	// Cache : éviter de relancer la même requête pour deux conditions identiques
	// dans une catégorie multi-conditions.
	cache := make(map[string]map[uint]int)

	for ci, cond := range cat.Conditions {
		key := condCacheKey(cond)
		scores, ok := cache[key]
		if !ok {
			scores = scoreCondition(db, groupID, memberIDs, now, cond)
			cache[key] = scores
		}
		for _, uid := range memberIDs {
			out[uid][ci] = scores[uid]
		}
	}
	return out
}

func condCacheKey(p config.ParsedRecipientPattern) string {
	return p.Raw
}

func scoreCondition(db *gorm.DB, groupID uint, memberIDs []uint, now time.Time, p config.ParsedRecipientPattern) map[uint]int {
	if p.IsEmailMatch {
		return userEmailMatch(db, groupID, p.Email)
	}
	if p.PerMonth {
		return userMonthScores(db, groupID, memberIDs, now, p)
	}
	return userTotalCounts(db, groupID, now.Add(-p.Window))
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

// BuildCategorySets retourne, pour chaque catégorie (par index), l'ensemble
// final des userIDs qui en font partie :
//
//   set(C) = primary(C) ∪ ⋃ primary(I) pour I ∈ C.Includes
//
// Où primary(X) = users dont la PREMIÈRE catégorie matchant (dans l'ordre du
// fichier) est X. Cette primary attribution est mutuellement exclusive entre
// catégories — c'est le comportement par défaut : un user matchant plusieurs
// patterns est primaire d'une seule catégorie. La directive `includes` permet
// à une catégorie d'agréger d'autres primaires explicitement.
//
// L'inclusion n'est pas récursive : seuls les primaires des catégories listées
// sont ajoutés. Les noms invalides dans Includes sont ignorés silencieusement
// (déjà loggés au boot).
func BuildCategorySets(db *gorm.DB, groupID uint, memberIDs []uint, now time.Time, cats []config.RecipientCategory) []map[uint]bool {
	nameToIdx := make(map[string]int, len(cats))

	// Phase 1 : own matchers par catégorie (qui matche le pattern brut).
	ownByIdx := make([]map[uint]bool, len(cats))
	for i, cat := range cats {
		nameToIdx[cat.Name] = i
		scores := UserCategoryScores(db, groupID, memberIDs, now, cat)
		own := make(map[uint]bool)
		for _, uid := range memberIDs {
			if cat.Match(scores[uid]) {
				own[uid] = true
			}
		}
		ownByIdx[i] = own
	}

	// Phase 2 : primary par catégorie (premier match gagne, mutuelle exclusivité).
	primaryByIdx := make([]map[uint]bool, len(cats))
	for i := range cats {
		primaryByIdx[i] = make(map[uint]bool)
	}
	for _, uid := range memberIDs {
		for i := range cats {
			if ownByIdx[i][uid] {
				primaryByIdx[i][uid] = true
				break
			}
		}
	}

	// Phase 3 : ensemble final = primary ∪ primaires des catégories incluses.
	out := make([]map[uint]bool, len(cats))
	for i, cat := range cats {
		final := make(map[uint]bool, len(primaryByIdx[i]))
		for uid := range primaryByIdx[i] {
			final[uid] = true
		}
		for _, incName := range cat.Includes {
			if incIdx, ok := nameToIdx[incName]; ok {
				for uid := range primaryByIdx[incIdx] {
					final[uid] = true
				}
			}
		}
		out[i] = final
	}
	return out
}

// EligibleUsersForCategory retourne les IDs des membres du groupe inclus
// dans l'ensemble final de la catégorie nommée — propres matchers + matchers
// des catégories listées dans Includes. Utilisé pour filtrer les destinataires
// des notifications cron.
func EligibleUsersForCategory(db *gorm.DB, groupID uint, now time.Time, allCats []config.RecipientCategory, target config.RecipientCategory) []uint {
	var memberIDs []uint
	db.Table("user_groups").Where("group_id = ?", groupID).Pluck("user_id", &memberIDs)
	if len(memberIDs) == 0 {
		return nil
	}
	sets := BuildCategorySets(db, groupID, memberIDs, now, allCats)
	for i, cat := range allCats {
		if cat.Name == target.Name {
			out := make([]uint, 0, len(sets[i]))
			for uid := range sets[i] {
				out = append(out, uid)
			}
			return out
		}
	}
	return nil
}

// userEmailMatch : map { user_id_du_membre_qui_matche : 1 }, vide sinon.
func userEmailMatch(db *gorm.DB, groupID uint, email string) map[uint]int {
	if email == "" {
		return map[uint]int{}
	}
	var userID uint
	db.Table("user_groups AS ug").
		Select("ug.user_id").
		Joins("JOIN users u ON u.id = ug.user_id").
		Where("ug.group_id = ? AND LOWER(u.email) = ?", groupID, email).
		Scan(&userID)
	if userID == 0 {
		return map[uint]int{}
	}
	return map[uint]int{userID: 1}
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
func userMonthScores(db *gorm.DB, groupID uint, memberIDs []uint, now time.Time, p config.ParsedRecipientPattern) map[uint]int {
	windowMonths := p.WindowMonths()
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
			if p.MatchPerMonthCount(cnt) {
				n++
			}
		}
		scores[uid] = n
	}
	return scores
}

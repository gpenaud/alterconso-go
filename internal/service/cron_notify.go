package service

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"time"

	"github.com/gpenaud/alterconso/internal/model"
	"github.com/gpenaud/alterconso/pkg/mailer"
)

// NotifyOrderOpenings envoie un email aux membres éligibles quand les commandes ouvrent.
// Fenêtre : order_start_date dans les 15 dernières minutes.
func (s *CronService) NotifyOrderOpenings() {
	now := time.Now()
	window := now.Add(-15 * time.Minute)

	var mds []model.MultiDistrib
	s.db.Preload("Group").Preload("Place").
		Where("order_start_date >= ? AND order_start_date <= ? AND NOT validated", window, now).
		Find(&mds)

	for _, md := range mds {
		if s.alreadySent(md.ID, model.NotifTypeOrderOpen) {
			continue
		}
		s.sendOrderNotification(md, model.NotifTypeOrderOpen)
	}
}

// NotifyOrderClosingSoon envoie un rappel 24h avant la fermeture des commandes.
// Fenêtre : order_end_date dans 23h45–24h15.
func (s *CronService) NotifyOrderClosingSoon() {
	now := time.Now()
	from := now.Add(23*time.Hour + 45*time.Minute)
	to := now.Add(24*time.Hour + 15*time.Minute)

	var mds []model.MultiDistrib
	s.db.Preload("Group").Preload("Place").
		Where("order_end_date >= ? AND order_end_date <= ? AND NOT validated", from, to).
		Find(&mds)

	for _, md := range mds {
		if s.alreadySent(md.ID, model.NotifTypeOrderClose24) {
			continue
		}
		s.sendOrderNotification(md, model.NotifTypeOrderClose24)
	}
}

// alreadySent vérifie si la notification a déjà été envoyée pour cette distribution.
func (s *CronService) alreadySent(multiDistribID uint, notifType string) bool {
	var count int64
	s.db.Model(&model.NotificationSent{}).
		Where("multi_distrib_id = ? AND type = ?", multiDistribID, notifType).
		Count(&count)
	return count > 0
}

// markSent enregistre qu'une notification a été envoyée.
func (s *CronService) markSent(multiDistribID uint, notifType string) {
	s.db.Create(&model.NotificationSent{
		MultiDistribID: multiDistribID,
		Type:           notifType,
		SentAt:         time.Now(),
	})
}

// sendOrderNotification envoie l'email aux membres éligibles et marque comme envoyé.
func (s *CronService) sendOrderNotification(md model.MultiDistrib, notifType string) {
	// Charger le groupe et le lieu si pas déjà chargés
	if md.Group.ID == 0 {
		s.db.First(&md.Group, md.GroupID)
	}
	if md.Place.ID == 0 && md.PlaceID != 0 {
		s.db.First(&md.Place, md.PlaceID)
	}
	users := s.eligibleUsers(md.GroupID, notifType)
	if len(users) == 0 {
		log.Printf("[NOTIFY] no eligible users for distrib %d type=%s", md.ID, notifType)
		s.markSent(md.ID, notifType)
		return
	}

	subject := emailSubject(md, notifType)
	sent := 0
	for _, u := range users {
		if s.dryRun {
			log.Printf("[DRY-RUN] email WOULD BE sent:\n  To: %s %s <%s>\n  Subject: %s",
				u.FirstName, u.LastName, u.Email, subject)
			continue
		}
		html, err := renderEmailTemplate(md, u, notifType, s.cfg.Host)
		if err != nil {
			log.Printf("[NOTIFY] template error user %d: %v", u.ID, err)
			continue
		}
		m := &mailer.Mail{
			From:     "",
			FromName: md.Group.Name,
			Subject:  subject,
			HTMLBody: html,
		}
		m.AddRecipient(u.Email, u.FirstName+" "+u.LastName)
		if err := s.mailer.Send(m); err != nil {
			log.Printf("[NOTIFY] send failed user %d: %v", u.ID, err)
		} else {
			sent++
		}
	}

	if s.dryRun {
		log.Printf("[DRY-RUN] distrib %d type=%s — %d emails auraient été envoyés (dry-run, aucun envoi)", md.ID, notifType, len(users))
	} else {
		log.Printf("[NOTIFY] distrib %d type=%s — %d/%d emails sent", md.ID, notifType, sent, len(users))
		s.markSent(md.ID, notifType)
	}
}

// eligibleUsers retourne les membres du groupe qui :
//  1. ont activé le flag de notification correspondant
//  2. matchent le pattern de la catégorie référencée dans
//     notifications.recipient_category (validée au boot, donc toujours présente).
func (s *CronService) eligibleUsers(groupID uint, notifType string) []model.User {
	var flag model.UserFlag
	switch notifType {
	case model.NotifTypeOrderOpen:
		flag = model.UserFlagEmailNotifOuverture
	case model.NotifTypeOrderClose24:
		flag = model.UserFlagEmailNotif24h
	default:
		return nil
	}

	cat := FindCategoryByName(s.cfg.Messages.RecipientCategories, s.cfg.Notifications.RecipientCategory)
	if cat == nil {
		// Ne devrait jamais arriver : la config est validée au boot.
		log.Printf("[NOTIFY] BUG: catégorie %q absente à l'exécution", s.cfg.Notifications.RecipientCategory)
		return nil
	}

	matchingIDs := EligibleUsersForCategory(s.db, groupID, time.Now(), *cat)
	if s.dryRun {
		log.Printf("[DRY-RUN] notifications filtrées par catégorie %q (%s) — %d users matchent le pattern",
			cat.Name, cat.Pattern, len(matchingIDs))
	}
	if len(matchingIDs) == 0 {
		return nil
	}

	var users []model.User
	s.db.Where("id IN ? AND (flags & ?) != 0", matchingIDs, uint(flag)).Find(&users)
	return users
}

func emailSubject(md model.MultiDistrib, notifType string) string {
	date := md.DistribStartDate.Format("02/01/2006")
	switch notifType {
	case model.NotifTypeOrderOpen:
		return fmt.Sprintf("[%s] Les commandes sont ouvertes — livraison du %s", md.Group.Name, date)
	case model.NotifTypeOrderClose24:
		return fmt.Sprintf("[%s] Rappel : commandes fermées dans 24h — livraison du %s", md.Group.Name, date)
	}
	return "Notification Alterconso"
}

// ---- Templates email ----

type emailData struct {
	GroupName    string
	UserName     string
	DistribDate  string
	DistribHour  string
	OrderEndDate string
	Place        string
	ShopURL      string
	NotifType    string
}

const emailTpl = `<!DOCTYPE html>
<html lang="fr">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.GroupName}}</title>
</head>
<body style="margin:0;padding:0;background:#f5f0e8;font-family:Arial,sans-serif;font-size:14px;color:#333;">
<table width="100%" cellpadding="0" cellspacing="0" style="background:#f5f0e8;padding:24px 0;">
  <tr>
    <td align="center">
      <table width="600" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:4px;overflow:hidden;box-shadow:0 1px 4px rgba(0,0,0,0.1);">

        <!-- Header -->
        <tr>
          <td style="background:#6a9a2a;padding:20px 30px;">
            <h1 style="margin:0;color:#fff;font-size:1.4em;font-weight:bold;">{{.GroupName}}</h1>
          </td>
        </tr>

        <!-- Body -->
        <tr>
          <td style="padding:28px 30px;">
            <p style="margin:0 0 16px;">Bonjour <strong>{{.UserName}}</strong>,</p>

            {{if eq .NotifType "order_open"}}
            <p style="margin:0 0 16px;">
              Les commandes pour la livraison du <strong>{{.DistribDate}}</strong> à <strong>{{.DistribHour}}</strong>
              {{if .Place}}au <strong>{{.Place}}</strong>{{end}} sont désormais <strong style="color:#6a9a2a;">ouvertes</strong>.
            </p>
            <p style="margin:0 0 24px;">
              Vous avez jusqu'au <strong>{{.OrderEndDate}}</strong> pour passer votre commande.
            </p>
            {{else}}
            <p style="margin:0 0 16px;">
              Rappel : les commandes pour la livraison du <strong>{{.DistribDate}}</strong> à <strong>{{.DistribHour}}</strong>
              {{if .Place}}au <strong>{{.Place}}</strong>{{end}} ferment dans <strong style="color:#c0392b;">24 heures</strong>.
            </p>
            <p style="margin:0 0 24px;">
              Ne tardez pas à passer votre commande avant le <strong>{{.OrderEndDate}}</strong>.
            </p>
            {{end}}

            <!-- CTA -->
            <table cellpadding="0" cellspacing="0" style="margin:0 0 24px;">
              <tr>
                <td style="background:#6a9a2a;border-radius:4px;">
                  <a href="{{.ShopURL}}" style="display:inline-block;padding:12px 28px;color:#fff;text-decoration:none;font-weight:bold;font-size:1em;">
                    Commander maintenant →
                  </a>
                </td>
              </tr>
            </table>

            <p style="margin:0;color:#888;font-size:0.85em;">
              Vous recevez cet email car vous avez activé les notifications de commande.
              Vous pouvez les désactiver dans votre <a href="{{.ShopURL}}" style="color:#6a9a2a;">espace personnel</a>.
            </p>
          </td>
        </tr>

        <!-- Footer -->
        <tr>
          <td style="background:#f9f9f9;padding:14px 30px;border-top:1px solid #eee;text-align:center;font-size:0.8em;color:#aaa;">
            {{.GroupName}} — propulsé par Alterconso
          </td>
        </tr>

      </table>
    </td>
  </tr>
</table>
</body>
</html>`

var emailTemplate = template.Must(template.New("email").Parse(emailTpl))

func renderEmailTemplate(md model.MultiDistrib, u model.User, notifType, host string) (string, error) {
	orderEnd := ""
	if md.OrderEndDate != nil {
		orderEnd = md.OrderEndDate.Format("02/01/2006 à 15:04")
	}
	place := ""
	if md.Place.ID != 0 {
		place = md.Place.Name
	}

	data := emailData{
		GroupName:    md.Group.Name,
		UserName:     u.FirstName,
		DistribDate:  md.DistribStartDate.Format("02/01/2006"),
		DistribHour:  md.DistribStartDate.Format("15h04"),
		OrderEndDate: orderEnd,
		Place:        place,
		ShopURL:      fmt.Sprintf("https://%s/home", host),
		NotifType:    notifType,
	}

	var buf bytes.Buffer
	if err := emailTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

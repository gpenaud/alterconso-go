package service

import (
	"bytes"
	"html/template"
	"log"
	"strconv"
	"strings"

	"github.com/gpenaud/alterconso/internal/filesign"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// CycleMessage : le courrier configuré pour un cycle, prêt à être rendu.
//
// Une vue et non le modèle : le service n'a pas à savoir où l'image est
// stockée ni comment son adresse se signe.
type CycleMessage struct {
	Subject           string
	Body              string
	ButtonLabel       string
	ImageURL          string
	RecipientCategory string
}

// cycleMessageFor retourne le courrier à envoyer pour une distribution.
//
// Nil si elle n'appartient à aucun cycle, si son cycle n'a pas de courrier, ou
// s'il n'est pas prêt à partir : le gabarit d'origine prend alors le relais.
// C'est ce qui fait qu'activer cette fonction ne prive de courrier aucune des
// distributions créées une par une.
func cycleMessageFor(db *gorm.DB, md model.MultiDistrib, key, host string) *CycleMessage {
	if md.CycleID == nil {
		return nil
	}
	var msg model.CycleMessage
	if err := db.Where("cycle_id = ?", *md.CycleID).First(&msg).Error; err != nil {
		return nil
	}
	if !msg.IsSendable() {
		return nil
	}

	out := &CycleMessage{
		Subject:           msg.Subject,
		Body:              msg.Body,
		ButtonLabel:       msg.ButtonLabel(),
		RecipientCategory: msg.RecipientCategory,
	}

	if msg.ImageFileID != nil {
		var f model.File
		if db.Select("id, name").First(&f, *msg.ImageFileID).Error == nil {
			// Adresse absolue : un chemin relatif ne mène nulle part depuis une
			// boîte mail, qui n'a aucune idée du domaine dont vient le message.
			out.ImageURL = "https://" + host + filesign.URL(f.ID, key, f.Name)
		} else {
			log.Printf("[NOTIFY] image %d introuvable pour le cycle %d", *msg.ImageFileID, *md.CycleID)
		}
	}
	return out
}

// cycleEmailTemplate rend le courrier configuré sur un cycle.
//
// Le texte passe par {{.Body}} et non par un template imbriqué : ce que le
// responsable écrit est du texte, jamais du gabarit. Sans cela, une accolade
// tapée par mégarde ferait échouer le rendu, et une expression volontaire
// donnerait accès aux données passées au modèle.
var cycleEmailTemplate = template.Must(template.New("cycle").Parse(`<!DOCTYPE html>
<html lang="fr"><head><meta charset="UTF-8"></head>
<body style="font-family:Arial,sans-serif;background:#f5f0e8;padding:24px;margin:0;">
<table width="100%" cellpadding="0" cellspacing="0"><tr><td align="center">
  <table width="560" style="background:#fff;border-radius:4px;overflow:hidden;">
    {{if .ImageURL}}
    <tr><td>
      <img src="{{.ImageURL}}" alt="" width="560" style="display:block;width:100%;max-width:560px;height:auto;border:0;" />
    </td></tr>
    {{end}}
    <tr><td style="background:#6a9a2a;padding:16px 30px;">
      <h1 style="margin:0;color:#fff;font-size:1.2em;">{{.GroupName}}</h1>
    </td></tr>
    <tr><td style="padding:28px 30px;color:#333;">
      <p style="margin:0 0 16px;">Bonjour {{.FirstName}},</p>
      <div style="margin:0 0 20px;line-height:1.5;">{{.Body}}</div>
      <p style="margin:0 0 4px;color:#555;">
        Distribution du <strong>{{.DistribDate}}</strong>{{if .PlaceName}} — {{.PlaceName}}{{end}}.
      </p>
      {{if .OrderEndDate}}
      <p style="margin:0 0 20px;color:#555;">Commandes ouvertes jusqu'au <strong>{{.OrderEndDate}}</strong>.</p>
      {{end}}
      <table cellpadding="0" cellspacing="0" style="margin:24px 0;">
        <tr><td style="background:#c1440e;border-radius:4px;">
          <a href="{{.ShopURL}}" style="display:inline-block;padding:12px 28px;color:#fff;text-decoration:none;font-weight:bold;">
            {{.ButtonLabel}}
          </a>
        </td></tr>
      </table>
      <p style="color:#888;font-size:0.85em;margin:0;">
        Vous recevez ce message parce que vous êtes membre de {{.GroupName}}.
        Vous pouvez régler vos notifications dans votre
        <a href="{{.AccountURL}}" style="color:#6a9a2a;">espace personnel</a>.
      </p>
    </td></tr>
  </table>
</td></tr></table>
</body></html>`))

// cycleEmailData : ce que le gabarit connaît. Body est du HTML déjà échappé,
// retours à la ligne convertis — le responsable écrit du texte, pas du balisage.
type cycleEmailData struct {
	FirstName    string
	GroupName    string
	Body         template.HTML
	ImageURL     string
	ButtonLabel  string
	ShopURL      string
	AccountURL   string
	DistribDate  string
	PlaceName    string
	OrderEndDate string
}

// renderCycleEmail compose le courrier d'un destinataire.
func renderCycleEmail(md model.MultiDistrib, u model.User, msg CycleMessage, host string) (string, error) {
	data := cycleEmailData{
		FirstName:   u.FirstName,
		GroupName:   md.Group.Name,
		Body:        textToHTML(msg.Body),
		ImageURL:    msg.ImageURL,
		ButtonLabel: msg.ButtonLabel,
		// Le sas plutôt que /shop/:id en direct : celui-ci répond 200 sans
		// session et laisse l'adhérent devant une page vide.
		ShopURL:     "https://" + host + "/distribution/order/" + strconv.FormatUint(uint64(md.ID), 10),
		AccountURL:  "https://" + host + "/account",
		DistribDate: md.DistribStartDate.Format("02/01/2006"),
		PlaceName:   md.Place.Name,
	}
	if md.OrderEndDate != nil {
		data.OrderEndDate = md.OrderEndDate.Format("02/01/2006 à 15h04")
	}

	var b bytes.Buffer
	if err := cycleEmailTemplate.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// textToHTML échappe le texte saisi puis rétablit ses retours à la ligne.
//
// L'échappement vient d'abord : sans lui, un responsable pourrait glisser du
// balisage — ou un lien — dans un courrier que le groupe signe.
func textToHTML(s string) template.HTML {
	escaped := template.HTMLEscapeString(strings.ReplaceAll(s, "\r\n", "\n"))
	return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
}

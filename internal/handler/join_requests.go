package handler

import (
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/middleware"
	"github.com/gpenaud/alterconso/internal/model"
	"github.com/gpenaud/alterconso/pkg/mailer"
	"gorm.io/gorm"
)

// ─── Groupes proposés à l'inscription ────────────────────────────────────────

// openForRegistrationGroups : les groupes qu'un nouvel inscrit peut demander.
//
// Ni fermés ni complets : ces deux modes disent que le groupe n'accueille
// personne en ce moment, et les proposer quand même reviendrait à faire
// remplir un formulaire pour un refus certain.
//
// Le mode « ouvert » ne dispense pas de l'accord d'un gestionnaire : il dit que
// le groupe cherche des adhérents, pas que n'importe qui y entre seul.
func openForRegistrationGroups(db *gorm.DB) []model.Group {
	var groups []model.Group
	db.Where("reg_option IN ?", []string{
		string(model.RegOptionOpen),
		string(model.RegOptionWaitingList),
	}).Order("name").Find(&groups)
	return groups
}

// isGroupOpenForRegistration garde le formulaire d'inscription : le groupe
// soumis arrive d'un champ de formulaire, donc de l'extérieur, et rien
// n'empêche d'y écrire l'identifiant d'un groupe fermé.
func isGroupOpenForRegistration(db *gorm.DB, groupID uint) bool {
	if groupID == 0 {
		return false
	}
	var count int64
	db.Model(&model.Group{}).
		Where("id = ? AND reg_option IN ?", groupID, []string{
			string(model.RegOptionOpen),
			string(model.RegOptionWaitingList),
		}).Count(&count)
	return count > 0
}

// ─── Dépôt de la demande ─────────────────────────────────────────────────────

// upsertJoinRequest enregistre la demande d'un candidat, ou remet en attente
// celle qu'il avait déjà déposée pour ce groupe.
//
// Réécrire plutôt qu'ajouter : le formulaire d'inscription se rejoue tant que
// le compte n'est pas activé — chaque renvoi du courrier d'activation repasse
// ici — et une demande par tentative encombrerait l'écran des gestionnaires de
// doublons.
func upsertJoinRequest(db *gorm.DB, userID, groupID uint, message *string) error {
	var req model.GroupJoinRequest
	err := db.Where("user_id = ? AND group_id = ?", userID, groupID).First(&req).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&model.GroupJoinRequest{
			UserID:  userID,
			GroupID: groupID,
			Status:  model.JoinRequestPending,
			Message: message,
		}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&req).Updates(map[string]any{
		"status":        model.JoinRequestPending,
		"message":       message,
		"decided_at":    nil,
		"decided_by_id": nil,
	}).Error
}

// ─── Destinataires ───────────────────────────────────────────────────────────

// membershipManagers : les membres à prévenir qu'une demande attend.
//
// Ceux qui portent « gestion des membres », et le responsable du groupe, qui
// détient ce droit sans qu'il figure dans sa liste. Le tri sur les droits se
// fait en Go et non en SQL : ils sont stockés en JSON dans une colonne texte,
// qu'aucun WHERE ne sait interroger de façon portable.
func membershipManagers(db *gorm.DB, groupID uint) []model.User {
	var members []model.UserGroup
	if err := db.Where("group_id = ?", groupID).Preload("User").Find(&members).Error; err != nil {
		return nil
	}

	var out []model.User
	for i := range members {
		m := &members[i]
		if receivesJoinRequests(m) {
			out = append(out, m.User)
		}
	}
	return out
}

// receivesJoinRequests : ce membre doit-il être prévenu qu'une demande attend ?
//
// La décision est isolée de la requête qui la nourrit pour être vérifiable
// sans base, comme authorize l'est pour les routes. Les délégations
// « distributions » et « paramètres » n'y donnent pas droit : elles ne
// décident de rien quant aux membres, et le courrier qu'elles recevraient ne
// leur demanderait qu'un geste qu'elles ne peuvent pas faire.
func receivesJoinRequests(ug *model.UserGroup) bool {
	if ug == nil {
		return false
	}
	return ug.HasRight(model.RightMembership) || ug.HasRight(model.RightGroupAdmin)
}

// joinRequestRecipients : à qui adresser le courrier d'une demande.
//
// Les gestionnaires des membres, et à défaut le responsable technique : un
// groupe qui n'a désigné personne laisserait sinon ses candidats attendre une
// décision que rien ne signale à qui que ce soit.
func joinRequestRecipients(db *gorm.DB, groupID uint) []model.User {
	managers := membershipManagers(db, groupID)
	if len(managers) > 0 {
		return managers
	}
	if technicalManagerEmail == "" {
		return nil
	}
	var fallback model.User
	if err := db.Where("email = ?", technicalManagerEmail).First(&fallback).Error; err != nil {
		return nil
	}
	return []model.User{fallback}
}

// ─── Courriers ───────────────────────────────────────────────────────────────

// notifyJoinRequest prévient les gestionnaires qu'une demande les attend.
//
// Le courrier renvoie vers l'écran de validation plutôt que de porter les
// boutons lui-même : un lien qui déciderait à lui seul vaudrait décision pour
// quiconque reçoit, transfère ou intercepte le message, alors que l'écran
// demande d'abord de s'authentifier.
func (h *PagesHandler) notifyJoinRequest(user model.User, group model.Group, message *string) {
	recipients := joinRequestRecipients(h.db, group.ID)
	if len(recipients) == 0 {
		log.Printf("[adhesion] demande de %s pour le groupe %d : aucun gestionnaire à prévenir",
			user.Email, group.ID)
		return
	}

	url := fmt.Sprintf("https://%s/member/requests", h.cfg.Host)
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr"><head><meta charset="UTF-8"></head>
<body style="font-family:Arial,sans-serif;background:#f5f0e8;padding:24px;">
<table width="100%%" cellpadding="0" cellspacing="0">
  <tr><td align="center">
    <table width="560" style="background:#fff;border-radius:4px;overflow:hidden;">
      <tr><td style="background:#6a9a2a;padding:20px 30px;">
        <h1 style="margin:0;color:#fff;font-size:1.3em;">Une demande d'adhésion attend</h1>
      </td></tr>
      <tr><td style="padding:28px 30px;">
        <p><strong>%s</strong> (%s) demande à rejoindre le groupe <strong>%s</strong>.</p>
        %s
        <p>Vous recevez ce message parce que vous gérez les membres de ce groupe.
           Il suffit que l'un d'entre vous tranche.</p>
        <table cellpadding="0" cellspacing="0" style="margin:24px 0;">
          <tr><td style="background:#6a9a2a;border-radius:4px;">
            <a href="%s" style="display:inline-block;padding:12px 28px;color:#fff;text-decoration:none;font-weight:bold;">
              Examiner la demande →
            </a>
          </td></tr>
        </table>
        <p style="color:#888;font-size:0.85em;">Le lien vous demandera de vous connecter.</p>
      </td></tr>
    </table>
  </td></tr>
</table>
</body></html>`, html.EscapeString(user.Name()), html.EscapeString(user.Email),
		html.EscapeString(group.Name), joinRequestMessageBlock(message), url)

	m := &mailer.Mail{
		From:     h.cfg.DefaultEmail,
		FromName: "Alterconso",
		Subject:  fmt.Sprintf("Demande d'adhésion : %s", user.Name()),
		HTMLBody: body,
	}
	for _, r := range recipients {
		m.AddRecipient(r.Email, r.FirstName+" "+r.LastName)
	}
	if err := h.mailer.Send(m); err != nil {
		log.Printf("[adhesion] envoi aux gestionnaires du groupe %d échoué : %v", group.ID, err)
	}
}

// joinRequestMessageBlock rend le mot du candidat, s'il en a laissé un.
func joinRequestMessageBlock(message *string) string {
	if message == nil || strings.TrimSpace(*message) == "" {
		return ""
	}
	return fmt.Sprintf(
		`<blockquote style="margin:16px 0;padding:10px 14px;border-left:3px solid #6a9a2a;color:#555;">%s</blockquote>`,
		html.EscapeString(strings.TrimSpace(*message)))
}

// notifyJoinDecision informe le candidat du sort de sa demande. Sans ce
// courrier, un compte accepté resterait sur l'écran « aucun groupe » jusqu'à ce
// que son titulaire pense à revenir voir.
func (h *PagesHandler) notifyJoinDecision(user model.User, group model.Group, accepted bool) {
	subject := fmt.Sprintf("Votre demande pour %s a été refusée", group.Name)
	inner := fmt.Sprintf(`<p>Bonjour <strong>%s</strong>,</p>
        <p>Votre demande pour rejoindre le groupe <strong>%s</strong> n'a pas été retenue.</p>
        <p>Si cette décision vous surprend, écrivez au groupe : elle a été prise par une personne, et rien n'y est définitif.</p>`,
		html.EscapeString(user.FirstName), html.EscapeString(group.Name))

	if accepted {
		subject = fmt.Sprintf("Bienvenue dans %s", group.Name)
		inner = fmt.Sprintf(`<p>Bonjour <strong>%s</strong>,</p>
        <p>Votre demande a été acceptée : vous êtes désormais membre du groupe <strong>%s</strong>.</p>
        <table cellpadding="0" cellspacing="0" style="margin:24px 0;">
          <tr><td style="background:#6a9a2a;border-radius:4px;">
            <a href="https://%s/home" style="display:inline-block;padding:12px 28px;color:#fff;text-decoration:none;font-weight:bold;">
              Voir les prochaines distributions →
            </a>
          </td></tr>
        </table>`,
			html.EscapeString(user.FirstName), html.EscapeString(group.Name), h.cfg.Host)
	}

	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="fr"><head><meta charset="UTF-8"></head>
<body style="font-family:Arial,sans-serif;background:#f5f0e8;padding:24px;">
<table width="100%%" cellpadding="0" cellspacing="0">
  <tr><td align="center">
    <table width="560" style="background:#fff;border-radius:4px;overflow:hidden;">
      <tr><td style="background:#6a9a2a;padding:20px 30px;">
        <h1 style="margin:0;color:#fff;font-size:1.3em;">%s</h1>
      </td></tr>
      <tr><td style="padding:28px 30px;">%s</td></tr>
    </table>
  </td></tr>
</table>
</body></html>`, html.EscapeString(subject), inner)

	m := &mailer.Mail{
		From:     h.cfg.DefaultEmail,
		FromName: "Alterconso",
		Subject:  subject,
		HTMLBody: body,
	}
	m.AddRecipient(user.Email, user.FirstName+" "+user.LastName)
	if err := h.mailer.Send(m); err != nil {
		log.Printf("[adhesion] réponse à %s échouée : %v", user.Email, err)
	}
}

// ─── Écran des demandes ──────────────────────────────────────────────────────

// JoinRequestEntry : une demande telle qu'elle s'affiche.
type JoinRequestEntry struct {
	ID      uint
	UserID  uint
	Name    string
	Email   string
	Phone   string
	City    string
	Date    string
	Message string
}

type JoinRequestsData struct {
	PageData
	Requests []JoinRequestEntry
}

// MemberRequestsPage liste les demandes d'adhésion en attente du groupe
// courant. Réservée aux gestionnaires des membres par le middleware de route ;
// le test ici est la seconde serrure, pour le cas où la route perdrait la
// sienne.
func (h *PagesHandler) MemberRequestsPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.HasMembership {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	requests := pendingJoinRequests(h.db, pd.Group.ID)

	data := JoinRequestsData{PageData: pd}
	data.Title = "Demandes d'adhésion"
	data.Category = "member"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Membres", Link: "/member"}}
	data.Flash, data.FlashError = joinRequestFlash(c.Query("done"), c.Query("who"))

	for _, r := range requests {
		entry := JoinRequestEntry{
			ID:     r.ID,
			UserID: r.UserID,
			Name:   r.User.Name(),
			Email:  r.User.Email,
			Date:   r.CreatedAt.Format("02/01/2006"),
		}
		if r.User.Phone != nil {
			entry.Phone = *r.User.Phone
		}
		if r.User.City != nil {
			entry.City = *r.User.City
		}
		if r.Message != nil {
			entry.Message = *r.Message
		}
		data.Requests = append(data.Requests, entry)
	}

	t, err := loadTemplates("base.html", "design.html", "member_requests.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// pendingJoinRequests : les demandes que le groupe doit trancher.
//
// La jointure écarte les comptes dont l'adresse n'est pas confirmée : leur
// demande existe dès le formulaire rempli, mais tant que le courrier
// d'activation n'a pas été suivi, rien ne prouve que l'adresse appartient au
// candidat — et un formulaire public suffirait à faire défiler des inconnus
// devant les gestionnaires.
func pendingJoinRequests(db *gorm.DB, groupID uint) []model.GroupJoinRequest {
	var requests []model.GroupJoinRequest
	db.Joins("JOIN users ON users.id = group_join_requests.user_id").
		Where("group_join_requests.group_id = ? AND group_join_requests.status = ? AND users.email_verified_at IS NOT NULL",
			groupID, model.JoinRequestPending).
		Preload("User").
		Order("group_join_requests.created_at ASC").
		Find(&requests)
	return requests
}

// pendingJoinRequestCount alimente le compteur de la barre latérale des
// membres : le nombre est ce qui fait revenir sur l'écran, un lien seul se
// laisse oublier.
func pendingJoinRequestCount(db *gorm.DB, groupID uint) int {
	var n int64
	db.Model(&model.GroupJoinRequest{}).
		Joins("JOIN users ON users.id = group_join_requests.user_id").
		Where("group_join_requests.group_id = ? AND group_join_requests.status = ? AND users.email_verified_at IS NOT NULL",
			groupID, model.JoinRequestPending).
		Count(&n)
	return int(n)
}

// errJoinRequestSettled : la demande avait déjà été tranchée. Ce n'est pas une
// panne, mais le résultat normal de deux gestionnaires devant le même écran —
// et l'écran le dit plutôt que d'annoncer un échec.
var errJoinRequestSettled = errors.New("demande déjà tranchée")

// MemberRequestDecide tranche une demande : accepter fait entrer le candidat
// dans le groupe, refuser clôt la demande sans l'effacer.
//
// En POST et non en GET : cette décision fait entrer quelqu'un dans un groupe,
// et le préchargement d'un lien par un navigateur ou un antivirus de
// messagerie suffirait à la déclencher.
func (h *PagesHandler) MemberRequestDecide(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.HasMembership {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}
	claims := middleware.GetClaims(c)

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.String(http.StatusBadRequest, "demande invalide")
		return
	}
	// Le verbe est lu strictement : hors « accept » et « refuse », on ne
	// décide rien. Sans ce test, une URL mal formée vaudrait refus.
	decision := c.Param("decision")
	if decision != "accept" && decision != "refuse" {
		c.String(http.StatusBadRequest, "décision inconnue")
		return
	}
	accept := decision == "accept"

	// La demande doit appartenir au groupe courant : sans ce filtre, un
	// identifiant tapé dans l'URL trancherait celle d'un autre groupe.
	var req model.GroupJoinRequest
	if err := h.db.Preload("User").Preload("Group").
		Where("id = ? AND group_id = ?", uint(id), pd.Group.ID).
		First(&req).Error; err != nil {
		c.Redirect(http.StatusFound, "/member/requests?done=gone")
		return
	}
	if !req.IsPending() {
		// Deux gestionnaires ont ouvert l'écran en même temps : le second
		// arrive après la décision du premier, et l'apprend plutôt que de la
		// rejouer.
		c.Redirect(http.StatusFound, "/member/requests?done=already")
		return
	}

	now := time.Now()
	status := model.JoinRequestRefused
	if accept {
		status = model.JoinRequestAccepted
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		// La demande se referme d'abord, et seulement si elle était encore
		// ouverte : c'est cette écriture conditionnelle qui départage deux
		// gestionnaires qui cliquent en même temps. Le test plus haut ne le
		// pouvait pas — entre sa lecture et cette écriture, l'autre a le temps
		// de trancher.
		res := tx.Model(&model.GroupJoinRequest{}).
			Where("id = ? AND status = ?", req.ID, model.JoinRequestPending).
			Updates(map[string]any{
				"status":        status,
				"decided_at":    now,
				"decided_by_id": claims.UserID,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errJoinRequestSettled
		}

		if accept {
			// Déjà membre : la décision se contente alors de clore la demande.
			var existing model.UserGroup
			if errors.Is(tx.Where("user_id = ? AND group_id = ?", req.UserID, req.GroupID).
				First(&existing).Error, gorm.ErrRecordNotFound) {
				return tx.Create(&model.UserGroup{
					UserID:  req.UserID,
					GroupID: req.GroupID,
					Rights:  "[]",
				}).Error
			}
		}
		return nil
	})
	if errors.Is(err, errJoinRequestSettled) {
		c.Redirect(http.StatusFound, "/member/requests?done=already")
		return
	}
	if err != nil {
		log.Printf("[adhesion] décision sur la demande %d échouée : %v", req.ID, err)
		c.Redirect(http.StatusFound, "/member/requests?done=failed")
		return
	}

	h.notifyJoinDecision(req.User, req.Group, accept)

	done := "refused"
	if accept {
		done = "accepted"
	}
	c.Redirect(http.StatusFound, "/member/requests?done="+done+"&who="+url.QueryEscape(req.User.Name()))
}

// joinRequestFlash compose le bandeau qui suit une décision.
//
// L'URL ne porte qu'un code et le nom concerné, jamais la phrase elle-même :
// un message repris tel quel de la barre d'adresse laisserait écrire n'importe
// quoi dans un bandeau que l'application signe.
func joinRequestFlash(done, who string) (string, bool) {
	name := strings.TrimSpace(who)
	if len(name) > 80 {
		name = name[:80]
	}
	switch done {
	case "accepted":
		return name + " est désormais membre du groupe.", false
	case "refused":
		return name + " a été informé du refus de sa demande.", false
	case "already":
		return "Cette demande a déjà été traitée.", true
	case "gone":
		return "Cette demande n'existe plus.", true
	case "failed":
		return "L'enregistrement a échoué : la demande est inchangée.", true
	}
	return "", false
}

// groupNameByID : le nom du groupe, ou une chaîne vide s'il a disparu entre
// l'affichage du formulaire et son envoi.
func groupNameByID(db *gorm.DB, groupID uint) string {
	var g model.Group
	if err := db.Select("id, name").First(&g, groupID).Error; err != nil {
		return ""
	}
	return g.Name
}

// notifyPendingJoinRequests prévient les gestionnaires des groupes qu'un compte
// vient de demander à rejoindre. Appelée à l'activation du compte, et non au
// dépôt de la demande : tant que l'adresse n'est pas confirmée, rien ne dit
// qu'elle appartient au candidat.
func (h *PagesHandler) notifyPendingJoinRequests(userID uint) {
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return
	}

	var requests []model.GroupJoinRequest
	h.db.Preload("Group").
		Where("user_id = ? AND status = ?", userID, model.JoinRequestPending).
		Find(&requests)

	for _, r := range requests {
		h.notifyJoinRequest(user, r.Group, r.Message)
	}
}

// PendingJoinRequestsFor : les groupes où ce compte attend une décision.
// Sert à l'écran de choix de groupe, qui annonçait « vous n'appartenez à aucun
// groupe » à un candidat dont la demande était pourtant partie.
func PendingJoinRequestsFor(db *gorm.DB, userID uint) []model.GroupJoinRequest {
	var requests []model.GroupJoinRequest
	db.Preload("Group").
		Where("user_id = ? AND status = ?", userID, model.JoinRequestPending).
		Order("created_at ASC").
		Find(&requests)
	return requests
}

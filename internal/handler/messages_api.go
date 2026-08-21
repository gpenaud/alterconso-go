package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/pkg/mailer"
)

// RecipientOptionView : un destinataire proposé à l'utilisateur courant.
type RecipientOptionView struct {
	Value string `json:"value"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// MessagesRecipients : GET /api/messages/recipients.
//
// La liste dépend des droits : un adhérent n'y trouve que son responsable de
// groupe et le responsable technique. Elle sert aussi de garde à l'envoi —
// une valeur absente d'ici est inenvoyable.
func (h *PagesHandler) MessagesRecipients(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "aucun groupe courant"})
		return
	}

	options, _ := h.buildScopedRecipients(pd, time.Now())
	vues := make([]RecipientOptionView, 0, len(options))
	for _, o := range options {
		vues = append(vues, RecipientOptionView{Value: o.Value, Name: o.Name, Count: o.Count})
	}
	c.JSON(http.StatusOK, gin.H{"recipients": vues})
}

// MessagesSend : POST /api/messages.
//
// Meme chemin que le formulaire : la resolution passe par la carte deja
// restreinte aux droits de l utilisateur, et non sur l ensemble des
// categories. C est ce qui empeche un adherent de poster `recipients=all` et
// d ecrire a tout le groupe.
func (h *PagesHandler) MessagesSend(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "aucun groupe courant"})
		return
	}

	var payload struct {
		Recipients string `json:"recipients"`
		Subject    string `json:"subject"`
		Body       string `json:"body"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "requete illisible"})
		return
	}

	sujet := strings.TrimSpace(payload.Subject)
	corps := strings.TrimSpace(payload.Body)
	if sujet == "" || corps == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sujet et message sont requis"})
		return
	}

	_, emailsParDestinataire := h.buildScopedRecipients(pd, time.Now())
	destinataires := emailsParDestinataire[strings.TrimSpace(payload.Recipients)]
	if len(destinataires) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "destinataire inconnu"})
		return
	}

	// L expediteur affiche est le groupe ; l adherent reste joignable par le
	// Reply-To et nomme en signature.
	nomExpediteur := strings.TrimSpace(pd.Group.Name)
	if nomExpediteur == "" {
		nomExpediteur = "Alterconso"
	}
	signature := strings.TrimSpace(pd.User.FirstName + " " + pd.User.LastName)
	corpsHTML := renderMessageHTML(pd.Group.Name, signature, pd.User.Email, h.cfg.Host, corps)

	envoyes, echecs := 0, 0
	for _, destinataire := range destinataires {
		m := &mailer.Mail{
			From:     h.cfg.DefaultEmail,
			FromName: nomExpediteur,
			ReplyTo:  pd.User.Email,
			Subject:  sujet,
			HTMLBody: corpsHTML,
		}
		m.AddRecipient(destinataire, "")
		if err := h.mailer.Send(m); err != nil {
			echecs++
			fmt.Printf("[MAIL] /api/messages echec vers %s : %v\n", destinataire, err)
		} else {
			envoyes++
		}
	}
	fmt.Printf("[MAIL] /api/messages sujet=%q envoyes=%d echecs=%d\n", sujet, envoyes, echecs)

	c.JSON(http.StatusOK, gin.H{"sent": envoyes, "failed": echecs})
}

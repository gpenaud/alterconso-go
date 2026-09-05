package handler

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// maxCycleImageSize borne l'image d'en-tête. Même limite que le logo : une
// image plus lourde ne s'afficherait pas mieux et alourdirait chaque envoi.
const maxCycleImageSize = 5 * 1024 * 1024

// cycleImageExtensions : ce qu'un client de messagerie sait afficher.
var cycleImageExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}

// ─── Liste des cycles ────────────────────────────────────────────────────────

// CycleRow : un cycle tel qu'il s'affiche dans la liste.
type CycleRow struct {
	ID           uint
	Name         string
	Place        string
	Period       string
	Rhythm       string
	NbDistribs   int
	NbUpcoming   int
	MessageState string
	HasMessage   bool
}

type CyclesData struct {
	PageData
	Cycles []CycleRow
}

// DistributionCyclesPage liste les cycles du groupe et l'état de leur courrier.
//
// Ouverte à la délégation « distributions » : c'est le calendrier des
// distributions qu'on administre ici, et le courrier qui l'accompagne.
func (h *PagesHandler) DistributionCyclesPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.HasDistributions {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	var cycles []model.DistributionCycle
	h.db.Preload("Place").
		Where("group_id = ?", pd.Group.ID).
		Order("start_date DESC").
		Find(&cycles)

	data := CyclesData{PageData: pd}
	data.Title = "Cycles de distribution"
	data.Category = "distribution"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Distributions", Link: "/distribution"}}
	data.Flash, data.FlashError = cycleFlash(c.Query("done"))

	now := time.Now()
	for _, cy := range cycles {
		var total, upcoming int64
		h.db.Model(&model.MultiDistrib{}).Where("cycle_id = ?", cy.ID).Count(&total)
		h.db.Model(&model.MultiDistrib{}).
			Where("cycle_id = ? AND distrib_start_date >= ?", cy.ID, now).Count(&upcoming)

		var msg model.CycleMessage
		hasMsg := h.db.Where("cycle_id = ?", cy.ID).First(&msg).Error == nil

		state := "Aucun courrier"
		switch {
		case hasMsg && msg.IsSendable():
			state = "Actif"
		case hasMsg:
			state = "Brouillon"
		}

		data.Cycles = append(data.Cycles, CycleRow{
			ID:           cy.ID,
			Name:         cy.Name,
			Place:        cy.Place.Name,
			Period:       cy.StartDate.Format("02/01/2006") + " → " + cy.EndDate.Format("02/01/2006"),
			Rhythm:       cy.RhythmLabel(),
			NbDistribs:   int(total),
			NbUpcoming:   int(upcoming),
			MessageState: state,
			HasMessage:   hasMsg,
		})
	}

	t, err := loadTemplates("base.html", "design.html", "distribution_cycles.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// cycleFlash compose le bandeau qui suit un enregistrement. Comme ailleurs,
// l'URL ne porte qu'un code : une phrase recopiée depuis la barre d'adresse
// s'afficherait dans un encadré que l'application signe.
func cycleFlash(done string) (string, bool) {
	switch done {
	case "saved":
		return "Le courrier a été enregistré.", false
	case "image-removed":
		return "L'image a été retirée.", false
	case "image-rejected":
		return "Image refusée : formats acceptés PNG, JPEG, GIF, WebP, jusqu'à 5 Mo.", true
	case "missing":
		return "Un objet et un texte sont nécessaires pour activer l'envoi.", true
	}
	return "", false
}

// ─── Configuration du courrier ───────────────────────────────────────────────

// loadOwnedCycle charge le cycle demandé en le bornant au groupe courant : un
// identifiant tapé dans l'URL ouvrirait sinon le courrier d'un autre groupe.
func (h *PagesHandler) loadOwnedCycle(c *gin.Context, groupID uint) (model.DistributionCycle, bool) {
	var cycle model.DistributionCycle
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.String(http.StatusBadRequest, "cycle invalide")
		return cycle, false
	}
	if err := h.db.Preload("Place").
		Where("id = ? AND group_id = ?", uint(id), groupID).
		First(&cycle).Error; err != nil {
		c.String(http.StatusNotFound, "cycle introuvable")
		return cycle, false
	}
	return cycle, true
}

// recipientCategoryNames : les catégories offertes au choix, telles que la
// configuration les nomme. La liste vient de là et non d'une constante : un
// groupe qui redéfinit ses catégories doit les voir ici sans qu'on recompile.
func (h *PagesHandler) recipientCategoryNames() []string {
	names := make([]string, 0, len(h.cfg.Messages.RecipientCategories))
	for _, cat := range h.cfg.Messages.RecipientCategories {
		names = append(names, cat.Name)
	}
	return names
}

// persistCycleMessage crée le courrier ou met à jour celui du cycle.
//
// Les champs sont énumérés plutôt que confiés à Save : celui-ci réécrirait
// image_file_id avec la valeur en mémoire, y compris quand le téléversement
// vient d'échouer et que le pointeur est resté nul.
func (h *PagesHandler) persistCycleMessage(msg *model.CycleMessage) error {
	if msg.ID == 0 {
		return h.db.Create(msg).Error
	}
	return h.db.Model(msg).Updates(map[string]any{
		"subject":            msg.Subject,
		"body":               msg.Body,
		"link_label":         msg.LinkLabel,
		"recipient_category": msg.RecipientCategory,
		"enabled":            msg.Enabled,
		"image_file_id":      msg.ImageFileID,
	}).Error
}

// ─── Sas vers la boutique ────────────────────────────────────────────────────

// DistributionOrderRedirect mène à la boutique d'une distribution, en
// s'assurant d'abord que le visiteur est connecté.
//
// C'est l'adresse que portent les courriers, et elle existe pour une raison
// précise : /shop/:id est servie par la SPA, qui répond 200 même sans session
// et n'échoue qu'ensuite, sur ses appels d'API. Un adhérent ouvrant le lien
// après l'expiration de son cookie — sept jours, alors qu'un courrier se lit
// souvent plus tard — tomberait sur une page vide sans qu'on lui propose de
// se connecter. Ici, PageAuth le renvoie vers la connexion en gardant sa
// destination, et il arrive sur la boutique après s'être identifié.
func (h *PagesHandler) DistributionOrderRedirect(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.Redirect(http.StatusFound, "/home")
		return
	}

	// Une distribution disparue depuis l'envoi renvoie à l'accueil plutôt qu'à
	// une boutique vide : le courrier a pu attendre des jours dans une boîte.
	var md model.MultiDistrib
	if h.db.Select("id").First(&md, uint(id)).Error != nil {
		c.Redirect(http.StatusFound, "/home")
		return
	}
	c.Redirect(http.StatusFound, "/shop/"+strconv.FormatUint(id, 10))
}

// ─── Génération des journées d'un cycle ──────────────────────────────────────

// cycleSchedule : les paramètres horaires d'un cycle, tels que le formulaire
// les recueille. Ils ne vivent pas sur le cycle lui-même : chaque journée porte
// ses propres dates, qu'un responsable peut décaler ensuite une à une.
type cycleSchedule struct {
	StartHour       string
	EndHour         string
	DaysBeforeOpen  int
	OpeningHour     string
	DaysBeforeClose int
	ClosingHour     string
}

// readSchedule lit les horaires du formulaire.
func readSchedule(c *gin.Context) (cycleSchedule, bool) {
	open, err1 := strconv.Atoi(c.PostForm("daysBeforeOpen"))
	close, err2 := strconv.Atoi(c.PostForm("daysBeforeClose"))
	s := cycleSchedule{
		StartHour:       c.PostForm("startHour"),
		EndHour:         c.PostForm("endHour"),
		DaysBeforeOpen:  open,
		OpeningHour:     c.PostForm("openingHour"),
		DaysBeforeClose: close,
		ClosingHour:     c.PostForm("closingHour"),
	}
	ok := err1 == nil && err2 == nil && s.StartHour != "" && s.EndHour != "" &&
		s.OpeningHour != "" && s.ClosingHour != ""
	return s, ok
}

// generateCycleDistributions crée les journées d'un cycle entre deux dates.
//
// Partagée par la création et la prolongation : c'est la même opération, et
// deux copies auraient divergé — l'une saluant les jours déjà pourvus, l'autre
// les doublant.
//
// Les jours déjà occupés sont sautés, non doublés : prolonger un cycle sur une
// période partiellement couverte remplirait sinon le calendrier de journées
// jumelles dont une seule serait jamais visible.
func (h *PagesHandler) generateCycleDistributions(
	cycle model.DistributionCycle, from, to time.Time, sch cycleSchedule,
) (created, skipped int) {
	for d := from; !d.After(to); d = cycle.Next(d) {
		day := d.Format("2006-01-02")
		distribStart, _ := time.ParseInLocation("2006-01-02T15:04", day+"T"+sch.StartHour, time.Local)
		distribEnd, _ := time.ParseInLocation("2006-01-02T15:04", day+"T"+sch.EndHour, time.Local)
		ordOpenDay := d.AddDate(0, 0, -sch.DaysBeforeOpen).Format("2006-01-02")
		ordCloseDay := d.AddDate(0, 0, -sch.DaysBeforeClose).Format("2006-01-02")
		ordOpen, _ := time.ParseInLocation("2006-01-02T15:04", ordOpenDay+"T"+sch.OpeningHour, time.Local)
		ordClose, _ := time.ParseInLocation("2006-01-02T15:04", ordCloseDay+"T"+sch.ClosingHour, time.Local)

		if h.multiDistribOn(cycle.GroupID, distribStart, 0) != nil {
			skipped++
			continue
		}

		cycleID := cycle.ID
		md := model.MultiDistrib{
			GroupID:          cycle.GroupID,
			PlaceID:          cycle.PlaceID,
			CycleID:          &cycleID,
			DistribStartDate: distribStart,
			DistribEndDate:   distribEnd,
			OrderStartDate:   &ordOpen,
			OrderEndDate:     &ordClose,
		}
		if h.db.Create(&md).Error == nil {
			created++
		}
	}
	return created, skipped
}

// intervalForCycleType traduit le rythme choisi du formulaire en périodicité.
//
// Retourne des jours pour les rythmes courts, des mois pour les longs : douze
// fois trente jours ne font pas un an, et un cycle annuel compté en jours
// décalerait d'une semaine à chaque tour.
func intervalForCycleType(t string) (days, months int) {
	switch t {
	case "Monthly":
		return 0, 1
	case "SemiAnnual":
		return 0, 6
	case "Annual":
		return 0, 12
	case "BiWeekly":
		return 14, 0
	case "TriWeekly":
		return 21, 0
	default:
		return 7, 0
	}
}

// ─── Création d'un cycle ─────────────────────────────────────────────────────

// CycleFormData sert les deux formulaires — création et modification — parce
// qu'ils posent les mêmes questions sur le courrier. Ce qui les sépare tient
// aux champs de programmation, que seule la création propose : les journées
// existantes ne se replanifient pas d'un bloc.
type CycleFormData struct {
	PageData
	Cycle      model.DistributionCycle
	Message    model.CycleMessage
	Places     []model.Place
	Categories []string
	ImageURL   string
	IsNew      bool
	// Valeurs par défaut du formulaire de création.
	DefaultStart string
	DefaultEnd   string
	// Modification seulement.
	RhythmText string
	NbDistribs int
	NbUpcoming int
	LastDate   string
}

// CycleNewPage crée un cycle : ses journées, et le courrier qui les annonce.
func (h *PagesHandler) CycleNewPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.HasDistributions {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	var places []model.Place
	h.db.Where("group_id = ?", pd.Group.ID).Find(&places)
	if len(places) == 0 {
		c.Redirect(http.StatusFound, "/distribution/cycles?done=no-place")
		return
	}

	if c.Request.Method == http.MethodPost {
		h.createCycle(c, pd, places)
		return
	}

	now := time.Now()
	data := CycleFormData{
		PageData:     pd,
		Places:       places,
		Categories:   h.recipientCategoryNames(),
		IsNew:        true,
		DefaultStart: now.Format("2006-01-02"),
		DefaultEnd:   now.AddDate(0, 3, 0).Format("2006-01-02"),
	}
	data.Title = "Nouveau cycle de distribution"
	data.Category = "distribution"
	data.Breadcrumb = []BreadcrumbItem{
		{Name: "Distributions", Link: "/distribution"},
		{Name: "Cycles de distribution", Link: "/distribution/cycles"},
		{Name: "Nouveau cycle", Link: ""},
	}
	data.Flash, data.FlashError = cycleFlash(c.Query("done"))
	h.renderCycleForm(c, data)
}

// createCycle enregistre le cycle, ses journées et son courrier.
func (h *PagesHandler) createCycle(c *gin.Context, pd PageData, places []model.Place) {
	startDate, err1 := time.ParseInLocation("2006-01-02", c.PostForm("startDate"), time.Local)
	endDate, err2 := time.ParseInLocation("2006-01-02", c.PostForm("endDate"), time.Local)
	placeID, err3 := strconv.ParseUint(c.PostForm("placeId"), 10, 64)
	sch, schOK := readSchedule(c)

	if err1 != nil || err2 != nil || err3 != nil || !schOK {
		c.Redirect(http.StatusFound, "/distribution/cycles/new?done=invalid")
		return
	}
	// Une fin antérieure au début ne produirait aucune journée, et le cycle
	// serait supprimé aussitôt créé sans que rien n'explique pourquoi.
	if endDate.Before(startDate) {
		c.Redirect(http.StatusFound, "/distribution/cycles/new?done=backwards")
		return
	}
	// Le lieu vient du formulaire : il doit appartenir au groupe.
	if !placeBelongsToGroup(places, uint(placeID)) {
		c.Redirect(http.StatusFound, "/distribution/cycles/new?done=invalid")
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		name = placeNameByID(places, uint(placeID))
	}
	days, months := intervalForCycleType(c.PostForm("cycleType"))

	cycle := model.DistributionCycle{
		GroupID:        pd.Group.ID,
		PlaceID:        uint(placeID),
		Name:           name,
		IntervalDays:   days,
		IntervalMonths: months,
		StartDate:      startDate,
		EndDate:        endDate,
	}
	if err := h.db.Create(&cycle).Error; err != nil {
		c.String(http.StatusInternalServerError, "impossible d'enregistrer le cycle")
		return
	}

	created, _ := h.generateCycleDistributions(cycle, startDate, endDate, sch)

	// Un cycle sans journée n'a rien à porter : sa période était déjà couverte.
	// Le conserver encombrerait la liste d'une entrée vide.
	if created == 0 {
		h.db.Delete(&cycle)
		c.Redirect(http.StatusFound, "/distribution/cycles?done=none-created")
		return
	}

	msg := model.CycleMessage{CycleID: cycle.ID}
	h.applyMessageForm(c, &msg)
	if err := h.persistCycleMessage(&msg); err != nil {
		log.Printf("[cycle] courrier du cycle %d non enregistré : %v", cycle.ID, err)
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/distribution/cycles?done=created&n=%d", created))
}

// placeBelongsToGroup : le lieu soumis figure-t-il parmi ceux du groupe ?
func placeBelongsToGroup(places []model.Place, id uint) bool {
	for _, p := range places {
		if p.ID == id {
			return true
		}
	}
	return false
}

// placeNameByID nomme le lieu, pour donner un nom au cycle qui n'en a pas.
func placeNameByID(places []model.Place, id uint) string {
	for _, p := range places {
		if p.ID == id {
			return p.Name
		}
	}
	return "Cycle"
}

// ─── Modification d'un cycle ─────────────────────────────────────────────────

// CycleEditPage modifie un cycle : son nom, son courrier, et sa prolongation.
//
// Ni la période ni le rythme des journées déjà créées ne s'y changent : elles
// portent des commandes, et les replanifier d'un bloc effacerait ce que des
// adhérents ont saisi. Prolonger ajoute des journées, sans toucher aux autres.
func (h *PagesHandler) CycleEditPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.HasDistributions {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	cycle, ok := h.loadOwnedCycle(c, pd.Group.ID)
	if !ok {
		return
	}

	var msg model.CycleMessage
	if err := h.db.Where("cycle_id = ?", cycle.ID).First(&msg).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusInternalServerError, "erreur: %v", err)
			return
		}
		msg = model.CycleMessage{CycleID: cycle.ID}
	}

	if c.Request.Method == http.MethodPost {
		h.updateCycle(c, cycle, &msg)
		return
	}

	data := CycleFormData{
		PageData:   pd,
		Cycle:      cycle,
		Message:    msg,
		Categories: h.recipientCategoryNames(),
		RhythmText: cycle.RhythmLabel(),
	}
	data.Title = "Cycle « " + cycle.Name + " »"
	data.Category = "distribution"
	data.Breadcrumb = []BreadcrumbItem{
		{Name: "Distributions", Link: "/distribution"},
		{Name: "Cycles de distribution", Link: "/distribution/cycles"},
		{Name: cycle.Name, Link: ""},
	}
	data.Flash, data.FlashError = cycleFlash(c.Query("done"))

	var total, upcoming int64
	h.db.Model(&model.MultiDistrib{}).Where("cycle_id = ?", cycle.ID).Count(&total)
	h.db.Model(&model.MultiDistrib{}).
		Where("cycle_id = ? AND distrib_start_date >= ?", cycle.ID, time.Now()).Count(&upcoming)
	data.NbDistribs, data.NbUpcoming = int(total), int(upcoming)

	// La dernière journée programmée : c'est d'elle que repart une
	// prolongation, et la connaître évite de proposer une date déjà couverte.
	var last model.MultiDistrib
	if h.db.Where("cycle_id = ?", cycle.ID).
		Order("distrib_start_date DESC").First(&last).Error == nil {
		data.LastDate = last.DistribStartDate.Format("02/01/2006")
	}

	if msg.ImageFileID != nil {
		var f model.File
		if h.db.Select("id, name").First(&f, *msg.ImageFileID).Error == nil {
			data.ImageURL = FileURL(f.ID, h.cfg.Key, f.Name)
		}
	}

	h.renderCycleForm(c, data)
}

// updateCycle enregistre le nom, le courrier, et prolonge si on le demande.
func (h *PagesHandler) updateCycle(c *gin.Context, cycle model.DistributionCycle, msg *model.CycleMessage) {
	dest := "/distribution/cycles/" + strconv.FormatUint(uint64(cycle.ID), 10)

	// Le retrait de l'image ne soumet pas un formulaire pour enregistrer un
	// texte, mais pour défaire un choix : il se traite avant tout le reste.
	if c.PostForm("removeImage") == "1" {
		h.removeCycleImage(msg)
		c.Redirect(http.StatusFound, dest+"?done=image-removed")
		return
	}

	if name := strings.TrimSpace(c.PostForm("name")); name != "" && name != cycle.Name {
		h.db.Model(&cycle).Update("name", name)
	}

	// Prolongation : on repart du lendemain de la dernière journée programmée,
	// en conservant le rythme. Les horaires viennent du formulaire, car ils ne
	// sont pas portés par le cycle.
	if newEnd := strings.TrimSpace(c.PostForm("extendTo")); newEnd != "" {
		if to, err := time.ParseInLocation("2006-01-02", newEnd, time.Local); err == nil {
			if sch, ok := readSchedule(c); ok {
				var last model.MultiDistrib
				from := cycle.StartDate
				if h.db.Where("cycle_id = ?", cycle.ID).
					Order("distrib_start_date DESC").First(&last).Error == nil {
					from = cycle.Next(last.DistribStartDate)
				}
				if !to.Before(from) {
					created, _ := h.generateCycleDistributions(cycle, from, to, sch)
					if created > 0 {
						h.db.Model(&cycle).Update("end_date", to)
						h.applyMessageForm(c, msg)
						if err := h.persistCycleMessage(msg); err != nil {
							log.Printf("[cycle] courrier du cycle %d non enregistré : %v", cycle.ID, err)
						}
						c.Redirect(http.StatusFound, fmt.Sprintf("%s?done=extended&n=%d", dest, created))
						return
					}
				}
				c.Redirect(http.StatusFound, dest+"?done=nothing-added")
				return
			}
		}
		c.Redirect(http.StatusFound, dest+"?done=invalid")
		return
	}

	h.applyMessageForm(c, msg)
	if msg.Enabled && (msg.Subject == "" || msg.Body == "") {
		msg.Enabled = false
		if err := h.persistCycleMessage(msg); err != nil {
			c.String(http.StatusInternalServerError, "erreur: %v", err)
			return
		}
		c.Redirect(http.StatusFound, dest+"?done=missing")
		return
	}

	rejected := h.attachCycleImage(c, msg)
	if err := h.persistCycleMessage(msg); err != nil {
		c.String(http.StatusInternalServerError, "erreur: %v", err)
		return
	}
	if rejected {
		c.Redirect(http.StatusFound, dest+"?done=image-rejected")
		return
	}
	c.Redirect(http.StatusFound, dest+"?done=saved")
}

// ─── Fonctions partagées par les deux formulaires ────────────────────────────

// applyMessageForm recueille le courrier depuis le formulaire.
//
// La case « envoyer un rappel » commande le tout : décochée, le texte est
// conservé mais rien ne part. C'est ce qui permet de préparer un courrier sans
// l'expédier, et de le suspendre sans l'effacer.
func (h *PagesHandler) applyMessageForm(c *gin.Context, msg *model.CycleMessage) {
	msg.Enabled = c.PostForm("enabled") == "1"
	msg.Subject = strings.TrimSpace(c.PostForm("subject"))
	msg.Body = strings.TrimSpace(c.PostForm("body"))
	msg.LinkLabel = strings.TrimSpace(c.PostForm("linkLabel"))
	msg.RecipientCategory = strings.TrimSpace(c.PostForm("recipientCategory"))

	// Une catégorie inconnue viderait le courrier de ses destinataires sans
	// rien dire : on n'accepte que celles que la configuration déclare.
	if msg.RecipientCategory != "" {
		known := false
		for _, name := range h.recipientCategoryNames() {
			if name == msg.RecipientCategory {
				known = true
				break
			}
		}
		if !known {
			msg.RecipientCategory = ""
		}
	}
}

// attachCycleImage enregistre l'image téléversée, s'il y en a une. Retourne
// vrai si un fichier a été présenté puis refusé — l'écran doit le dire, faute
// de quoi le responsable croirait son image en place.
func (h *PagesHandler) attachCycleImage(c *gin.Context, msg *model.CycleMessage) bool {
	fh, err := c.FormFile("image")
	if err != nil || fh == nil {
		return false
	}

	name := strings.ToLower(fh.Filename)
	allowed := false
	for _, ext := range cycleImageExtensions {
		if strings.HasSuffix(name, ext) {
			allowed = true
			break
		}
	}
	if !allowed || fh.Size > maxCycleImageSize {
		return true
	}

	src, err := fh.Open()
	if err != nil {
		return true
	}
	defer src.Close()
	data, err := io.ReadAll(src)
	if err != nil {
		return true
	}

	f := model.File{Name: fh.Filename, Data: data}
	if h.db.Create(&f).Error != nil {
		return true
	}
	old := msg.ImageFileID
	msg.ImageFileID = &f.ID
	if old != nil {
		h.db.Delete(&model.File{}, *old)
	}
	return false
}

// removeCycleImage détache l'image et la supprime : conservée, elle resterait
// servie par son adresse signée sans que rien n'y renvoie.
func (h *PagesHandler) removeCycleImage(msg *model.CycleMessage) {
	if msg.ImageFileID == nil {
		return
	}
	old := *msg.ImageFileID
	msg.ImageFileID = nil
	if msg.ID != 0 {
		h.db.Model(msg).Update("image_file_id", nil)
	}
	h.db.Delete(&model.File{}, old)
}

// renderCycleForm rend le formulaire, commun à la création et à la modification.
func (h *PagesHandler) renderCycleForm(c *gin.Context, data CycleFormData) {
	t, err := loadTemplates("base.html", "design.html", "distribution_cycle_form.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ─── Suppression d'un cycle ──────────────────────────────────────────────────

// multiDistribHasOrders : cette journée porte-t-elle des commandes ?
//
// Les paniers comptent autant que les commandes : en mode boutique, c'est par
// eux que passe ce qu'un adhérent a validé.
func (h *PagesHandler) multiDistribHasOrders(mdID uint) bool {
	var n int64
	h.db.Model(&model.UserOrder{}).
		Joins("JOIN distributions ON distributions.id = user_orders.distribution_id").
		Where("distributions.multi_distrib_id = ?", mdID).Count(&n)
	if n > 0 {
		return true
	}
	h.db.Model(&model.Basket{}).Where("multi_distrib_id = ?", mdID).Count(&n)
	return n > 0
}

// CycleDeleteAction supprime un cycle, son courrier et les journées qu'il a
// créées — sauf celles où des adhérents ont commandé.
//
// Ces dernières sont détachées, non détruites : effacer une journée commandée
// ferait disparaître ce que des gens ont saisi, et parfois payé. Elle reste au
// calendrier, sans cycle, et retrouve donc le message d'ouverture par défaut.
//
// En POST : cette action détruit des journées de calendrier, et le
// préchargement d'un lien suffirait à la déclencher.
func (h *PagesHandler) CycleDeleteAction(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil || !pd.HasDistributions {
		c.String(http.StatusForbidden, "accès refusé")
		return
	}

	cycle, ok := h.loadOwnedCycle(c, pd.Group.ID)
	if !ok {
		return
	}

	var days []model.MultiDistrib
	h.db.Select("id").Where("cycle_id = ?", cycle.ID).Find(&days)

	deleted, kept := 0, 0
	for _, md := range days {
		if h.multiDistribHasOrders(md.ID) {
			// Détachée plutôt que supprimée : la journée survit au cycle.
			h.db.Model(&model.MultiDistrib{}).Where("id = ?", md.ID).
				Update("cycle_id", nil)
			kept++
			continue
		}
		h.db.Where("multi_distrib_id = ?", md.ID).Delete(&model.Distribution{})
		h.db.Delete(&model.MultiDistrib{}, md.ID)
		deleted++
	}

	// Le courrier et son image partent avec le cycle : conservée, l'image
	// resterait servie par son adresse signée sans que rien n'y renvoie.
	var msg model.CycleMessage
	if h.db.Where("cycle_id = ?", cycle.ID).First(&msg).Error == nil {
		if msg.ImageFileID != nil {
			h.db.Delete(&model.File{}, *msg.ImageFileID)
		}
		h.db.Delete(&msg)
	}
	h.db.Delete(&cycle)

	log.Printf("[cycle] cycle %d supprimé : %d journées effacées, %d conservées car commandées",
		cycle.ID, deleted, kept)

	if kept > 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("/distribution/cycles?done=deleted-kept&n=%d", kept))
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/distribution/cycles?done=deleted&n=%d", deleted))
}

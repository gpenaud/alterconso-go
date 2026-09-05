package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gpenaud/alterconso/internal/model"
	"gorm.io/gorm"
)

// Ecriture des fiches producteurs.
//
// L'ecran des producteurs ne savait que lire. Les fiches se creaient par
// l'API JSON — POST /api/groups/:id/vendors, PUT /api/vendors/:id — qu'aucun
// ecran n'appelle : dans les faits, un groupe ne pouvait ni ajouter un
// fournisseur, ni corriger un numero de telephone sans passer par la base.

// VendorFormData : le formulaire de creation comme celui de modification.
// Un seul gabarit pour les deux, parce que ce sont les memes champs et que
// deux fichiers auraient diverge a la premiere correction.
type VendorFormData struct {
	PageData
	Vendor model.Vendor
	// Creation : l'ecran s'intitule et s'adresse differemment selon qu'on
	// ouvre une fiche ou qu'on la reprend.
	Creation bool
	// Erreur : ce qui a empeche l'enregistrement, reaffiche au-dessus du
	// formulaire avec ce qui avait ete saisi — le renvoyer vide obligerait a
	// tout retaper pour un courriel mal ecrit.
	Erreur string
	// Statuts : les statuts juridiques, libelles en francais. Le modele les
	// stocke en anglais, heritage de l'application d'origine.
	Statuts []VendorLegalStatusOption
	// Catalogs : ce qui empeche la suppression, et ce qui la permet quand la
	// liste est vide.
	Catalogs []model.Catalog
}

type VendorLegalStatusOption struct {
	Code  model.LegalStatus
	Label string
}

var vendorLegalStatuses = []VendorLegalStatusOption{
	{model.LegalStatusSoletrader, "Exploitation individuelle"},
	{model.LegalStatusOrganization, "Association"},
	{model.LegalStatusBusiness, "Societe"},
}

// vendorEcrivable : ce visiteur peut-il toucher a cette fiche ?
//
// Deux conditions, et les deux comptent. Le droit d'abord — le responsable de
// groupe, personne d'autre. Le rattachement ensuite : une fiche producteur ne
// porte pas de groupe, et sans cette seconde condition un responsable
// pourrait, en changeant le chiffre de l'adresse, reecrire la fiche d'une
// ferme qui ne livre que le groupe d'a cote. Relevent du groupe les
// producteurs qui lui tiennent un catalogue, et ceux qu'il a saisis.
func (h *PagesHandler) vendorEcrivable(pd PageData, vendorID uint) bool {
	if !pd.CanEditVendors || pd.Group == nil {
		return false
	}
	var n int64
	h.db.Model(&model.Catalog{}).
		Where("vendor_id = ? AND group_id = ?", vendorID, pd.Group.ID).
		Count(&n)
	if n > 0 {
		return true
	}
	var v model.Vendor
	if err := h.db.First(&v, vendorID).Error; err != nil {
		return false
	}
	return v.GroupID != nil && *v.GroupID == pd.Group.ID
}

// vendorDepuisFormulaire lit les champs postes dans la fiche donnee, et dit ce
// qui manque. Partage par la creation et la modification : la validation d'un
// courriel ne doit pas dependre de l'ecran d'ou il vient.
func vendorDepuisFormulaire(c *gin.Context, v *model.Vendor) string {
	texte := func(nom string) *string {
		s := strings.TrimSpace(c.PostForm(nom))
		if s == "" {
			return nil
		}
		return &s
	}

	v.Name = strings.TrimSpace(c.PostForm("name"))
	v.Email = strings.TrimSpace(c.PostForm("email"))
	v.Phone = texte("phone")
	v.Address1 = texte("address1")
	v.ZipCode = texte("zipCode")
	v.City = texte("city")
	v.Description = texte("description")
	v.Organic = c.PostForm("organic") == "1"

	v.LegalStatus = nil
	if code := strings.TrimSpace(c.PostForm("legalStatus")); code != "" {
		for _, s := range vendorLegalStatuses {
			if string(s.Code) == code {
				statut := s.Code
				v.LegalStatus = &statut
				break
			}
		}
	}

	if v.Name == "" {
		return "Le nom du producteur est obligatoire."
	}
	// Le courriel sert a joindre la ferme, et c'est le seul moyen de
	// contact que la fiche impose. Verifie sommairement : un « @ » entoure
	// de quelque chose. Le refuser sur une regle plus fine ferait echouer
	// des adresses valides, et l'envoi dira mieux que nous si elle existe.
	if v.Email == "" {
		return "Le courriel du producteur est obligatoire."
	}
	if at := strings.Index(v.Email, "@"); at <= 0 || at == len(v.Email)-1 || strings.Contains(v.Email, " ") {
		return "Ce courriel ne ressemble pas a une adresse : " + v.Email
	}
	return ""
}

func (h *PagesHandler) rendVendorForm(c *gin.Context, data VendorFormData) {
	data.Statuts = vendorLegalStatuses
	// Meme rubrique et meme largeur que la fiche : le menu lateral reste, et
	// la colonne ne saute pas d'un ecran a l'autre.
	data.Category = "contract"
	data.Container = "container-fluid ac-accueil"
	t, err := loadTemplates("base.html", "design.html", "cycles_style.html", "vendor_edit.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "template error: %v", err)
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base", data); err != nil {
		c.String(http.StatusInternalServerError, "render error: %v", err)
	}
}

// ---- GET/POST /vendor/insert ----

func (h *PagesHandler) VendorInsertPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}
	if !pd.CanEditVendors {
		c.Redirect(http.StatusFound, "/amap")
		return
	}

	data := VendorFormData{PageData: pd, Creation: true}
	data.Title = "Nouveau producteur"
	data.Breadcrumb = []BreadcrumbItem{{Name: "Producteurs", Link: "/amap"},
		{Name: "Nouveau producteur", Link: ""}}

	if c.Request.Method == http.MethodPost {
		if erreur := vendorDepuisFormulaire(c, &data.Vendor); erreur != "" {
			data.Erreur = erreur
			h.rendVendorForm(c, data)
			return
		}
		// Le groupe qui saisit garde la fiche a l'oeil tant qu'aucun
		// catalogue ne la rattache : sans cela elle n'apparaitrait nulle
		// part des l'enregistrement fait.
		groupID := pd.Group.ID
		data.Vendor.GroupID = &groupID
		if err := h.db.Create(&data.Vendor).Error; err != nil {
			data.Erreur = "L'enregistrement a echoue : " + err.Error()
			h.rendVendorForm(c, data)
			return
		}
		c.Redirect(http.StatusFound, "/vendor/view/"+strconv.FormatUint(uint64(data.Vendor.ID), 10))
		return
	}

	h.rendVendorForm(c, data)
}

// ---- GET/POST /vendor/edit/:id ----

func (h *PagesHandler) VendorEditPage(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	if !h.vendorEcrivable(pd, uint(id)) {
		c.Redirect(http.StatusFound, "/amap")
		return
	}

	var vendor model.Vendor
	if err := h.db.First(&vendor, id).Error; err != nil {
		c.String(http.StatusNotFound, "producteur introuvable")
		return
	}

	data := VendorFormData{PageData: pd, Vendor: vendor}
	// Tous groupes confondus, comme la garde de la suppression : c'est sur
	// cette liste que le gabarit decide d'offrir le bouton, et la proposer
	// pour la refuser ensuite ne vaudrait pas mieux que de ne rien dire.
	h.db.Where("vendor_id = ?", id).Find(&data.Catalogs)
	data.Title = "Modifier — " + vendor.Name
	data.Breadcrumb = []BreadcrumbItem{{Name: "Producteurs", Link: "/amap"},
		{Name: vendor.Name, Link: "/vendor/view/" + c.Param("id")},
		{Name: "Modifier", Link: ""}}

	if c.Request.Method == http.MethodPost {
		if erreur := vendorDepuisFormulaire(c, &data.Vendor); erreur != "" {
			data.Erreur = erreur
			h.rendVendorForm(c, data)
			return
		}
		// Champ par champ, et non « Save » sur la structure entiere : le
		// groupe createur ne se reecrit pas au passage — une fiche que le
		// voisin a saisie, et qu'un catalogue nous rend modifiable, reste
		// la sienne — et les valeurs vidangees doivent passer a NULL, ce
		// qu'un Updates sur structure ignorerait.
		if err := h.db.Model(&model.Vendor{}).Where("id = ?", id).
			Updates(map[string]interface{}{
				"name":         data.Vendor.Name,
				"email":        data.Vendor.Email,
				"phone":        data.Vendor.Phone,
				"address1":     data.Vendor.Address1,
				"zip_code":     data.Vendor.ZipCode,
				"city":         data.Vendor.City,
				"description":  data.Vendor.Description,
				"legal_status": data.Vendor.LegalStatus,
				"organic":      data.Vendor.Organic,
			}).Error; err != nil {
			data.Erreur = "L'enregistrement a echoue : " + err.Error()
			h.rendVendorForm(c, data)
			return
		}
		c.Redirect(http.StatusFound, "/vendor/view/"+c.Param("id"))
		return
	}

	h.rendVendorForm(c, data)
}

// ---- POST /vendor/delete/:id ----
//
// En POST : la suppression efface une fiche que d'autres groupes peuvent
// lire, et le prechargement d'un lien par un navigateur suffirait a la
// declencher.
func (h *PagesHandler) VendorDelete(c *gin.Context) {
	pd := h.buildPageData(c)
	if pd.User == nil || pd.Group == nil {
		c.Redirect(http.StatusFound, "/user/choose")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "id invalide")
		return
	}
	if !h.vendorEcrivable(pd, uint(id)) {
		c.Redirect(http.StatusFound, "/amap")
		return
	}

	// Un catalogue qui perdrait son producteur laisserait ses produits sans
	// fournisseur et ses commandes sans nom a imprimer. La fiche ne part
	// qu'une fois les catalogues retires — tous groupes confondus : celui
	// d'a cote commande peut-etre encore chez elle.
	var attaches int64
	h.db.Model(&model.Catalog{}).Where("vendor_id = ?", id).Count(&attaches)
	if attaches > 0 {
		var vendor model.Vendor
		if err := h.db.First(&vendor, id).Error; err != nil {
			c.String(http.StatusNotFound, "producteur introuvable")
			return
		}
		data := VendorFormData{PageData: pd, Vendor: vendor}
		h.db.Where("vendor_id = ?", id).Find(&data.Catalogs)
		data.Erreur = "Ce producteur tient encore des catalogues : retirez-les avant de supprimer sa fiche."
		data.Title = "Modifier — " + vendor.Name
		data.Breadcrumb = []BreadcrumbItem{{Name: "Producteurs", Link: "/amap"},
			{Name: vendor.Name, Link: "/vendor/view/" + c.Param("id")},
			{Name: "Modifier", Link: ""}}
		h.rendVendorForm(c, data)
		return
	}

	if err := h.db.Delete(&model.Vendor{}, id).Error; err != nil && err != gorm.ErrRecordNotFound {
		c.String(http.StatusInternalServerError, "suppression impossible: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/amap")
}

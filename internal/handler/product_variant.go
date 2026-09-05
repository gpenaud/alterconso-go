package handler

import (
	"regexp"
	"strings"
)

// quantiteOuPoids reconnaît ce qui distingue deux conditionnements d'un même
// produit : un nombre, éventuellement suivi d'une unité.
//
// « POULET BIO 1.8 à 1.9kg » et « POULET BIO 1.5 à 1.6kg » sont le même poulet
// vendu en deux calibres. Les montrer tous deux dans une bande de huit
// vignettes revient à gâcher une place pour la même photographie.
var quantiteOuPoids = regexp.MustCompile(
	`(?i)\b\d+([.,]\d+)?\s*(kg|kilos?|gr?|grammes?|l|cl|ml|litres?|cs|pi[eè]ces?|parts?|pers\.?|x)?\b`)

// separateurs : ce qui reste une fois les nombres retirés — tirets isolés,
// « à », « de », ponctuation — et qui ne distingue pas deux produits.
var separateurs = regexp.MustCompile(`(?i)\s*[\-–—/,;:()]+\s*|\s+\b(a|à|de|du|en|le|la|les|sous|vide)\b\s+`)

var espacesMultiples = regexp.MustCompile(`\s+`)

// productVariantKey réduit un nom de produit à ce qui l'identifie vraiment,
// conditionnement mis à part.
//
// Volontairement grossière : elle sert à écarter des doublons visuels dans un
// aperçu, pas à fusionner des références. Deux produits réellement différents
// qui tomberaient sur la même clé perdraient une vignette — désagrément mineur
// devant celui de voir trois fois le même poulet.
func productVariantKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = quantiteOuPoids.ReplaceAllString(s, " ")
	s = separateurs.ReplaceAllString(s, " ")
	s = espacesMultiples.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// dedupeVariants ne garde qu'un conditionnement par produit.
//
// Appliquée producteur par producteur : deux fermes peuvent vendre chacune un
// poulet, et montrer les deux dit justement qu'on a le choix. C'est le même
// poulet répété qui n'apprend rien.
func dedupeVariants(produits []ProductImageView) []ProductImageView {
	if len(produits) < 2 {
		return produits
	}
	vus := make(map[string]bool, len(produits))
	out := make([]ProductImageView, 0, len(produits))
	for _, p := range produits {
		cle := productVariantKey(p.Name)
		// Un nom réduit à rien — « 500 g » et guère plus — ne peut servir de
		// clé : on garde le produit plutôt que de confondre tous ceux-là.
		if cle != "" {
			if vus[cle] {
				continue
			}
			vus[cle] = true
		}
		out = append(out, p)
	}
	return out
}

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
// Le multiplicateur précède parfois le nombre — « Œufs x6 » — et le suit
// parfois — « 6 x 250 g ». Les deux formes disent la même chose.
var quantiteOuPoids = regexp.MustCompile(
	`(?i)\bx?\s*\d+([.,]\d+)?\s*(kg|kilos?|gr?|grammes?|l|cl|ml|litres?|cs|pi[eè]ces?|parts?|pers\.?|x)?\b`)

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

// motsVides : ce qui ne distingue pas deux produits.
var motsVides = map[string]bool{
	"de": true, "du": true, "des": true, "la": true, "le": true, "les": true,
	"au": true, "aux": true, "a": true, "à": true, "en": true, "et": true,
	"d": true, "l": true, "sous": true, "vide": true, "bio": true,
}

// productFamilyKey rapproche les déclinaisons d'un même produit : « RILLETTE
// DE POULET AU THYM » et « RILLETTE DE POULET NATURE », « Tisane paysanne —
// Éveil de printemps » et « … Évasion nocturne ».
//
// Les deux premiers mots qui portent du sens. C'est volontairement grossier, et
// cela ne sert qu'à composer un aperçu de huit vignettes : deux parfums d'une
// même tisane s'y ressemblent trop pour mériter deux places.
//
// Rien n'est masqué pour autant — le volet du producteur montre tous ses
// produits, et les catalogues sont intacts. Seule la bande arbitre.
//
// Un nom de moins de trois mots reste entier : « Jus de Caseille » et « Jus de
// pomme » se distinguent par leur troisième mot, et les confondre appauvrirait
// l'aperçu au lieu de le varier.
func productFamilyKey(name string) string {
	champs := strings.Fields(productVariantKey(name))
	utiles := make([]string, 0, 3)
	for _, m := range champs {
		if !motsVides[m] {
			utiles = append(utiles, m)
		}
	}
	if len(utiles) < 3 {
		return strings.Join(utiles, " ")
	}
	return strings.Join(utiles[:2], " ")
}

// dedupeFamilies ne garde qu'une déclinaison par famille de produits.
//
// Appliquée à la seule bande de l'accueil, après la déduplication des
// conditionnements : celle-ci est sûre, celle-là est un arbitrage d'aperçu.
func dedupeFamilies(produits []ProductImageView) []ProductImageView {
	if len(produits) < 2 {
		return produits
	}
	vus := make(map[string]bool, len(produits))
	out := make([]ProductImageView, 0, len(produits))
	for _, p := range produits {
		cle := productFamilyKey(p.Name)
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

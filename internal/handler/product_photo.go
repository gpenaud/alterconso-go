package handler

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"sync"

	"github.com/gpenaud/alterconso/internal/model"
)

// minPaletteColors : en deçà, l'image est un dessin, pas une photographie.
//
// Les PNG en palette indexée se répartissent nettement : les pictogrammes
// tiennent en quelques teintes — l'illustration générique de l'application en
// compte seize, et l'un des dessins importés neuf — tandis qu'une photo
// convertie en palette en utilise plus d'une centaine. Le seuil sépare les
// deux familles sans les frôler.
//
// Le poids du fichier ne suffisait pas : entre huit et quinze kilo-octets se
// mêlent des dessins et de vraies photographies de bocaux sur fond blanc, qui
// compressent tout aussi bien.
const minPaletteColors = 32

// looksLikePhoto : cet en-tête PNG est-il celui d'une photographie ?
//
// On ne lit que le début du fichier — les blocs d'en-tête précèdent toujours
// les données — ce qui évite de charger des images de deux cents kilo-octets
// pour n'en juger que la nature.
//
// Dans le doute, on répond oui : écarter à tort la photo d'un producteur est
// plus dommageable que laisser passer un dessin.
func looksLikePhoto(header []byte) bool {
	const signature = "\x89PNG\r\n\x1a\n"
	if len(header) < len(signature) || string(header[:len(signature)]) != signature {
		// JPEG, WebP ou autre : ces formats ne servent pas aux pictogrammes,
		// qui sont invariablement des PNG.
		return true
	}

	pos := len(signature)
	for pos+8 <= len(header) {
		length := int(binary.BigEndian.Uint32(header[pos : pos+4]))
		kind := string(header[pos+4 : pos+8])

		switch kind {
		case "PLTE":
			// Une entrée de palette occupe trois octets.
			return length/3 >= minPaletteColors
		case "IDAT":
			// Les données commencent sans qu'aucune palette n'ait paru :
			// l'image est en couleurs vraies, donc une photographie.
			return true
		}

		if length < 0 {
			return true
		}
		pos += 12 + length
	}
	// En-tête tronqué : on n'a pas de quoi juger, on laisse passer.
	return true
}

var photoVerdicts sync.Map // map[uint]bool

func PhotoVerdictCached(fileID uint) (bool, bool) {
	v, ok := photoVerdicts.Load(fileID)
	if !ok {
		return false, false
	}
	b, _ := v.(bool)
	return b, true
}

func RememberPhotoVerdict(fileID uint, isPhoto bool) {
	photoVerdicts.Store(fileID, isPhoto)
}

func IsPhotograph(data []byte) bool {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return true
	}
	b := img.Bounds()
	if b.Dx() < dominantSample || b.Dy() < dominantSample {
		return true
	}

	// Les couleurs sont ramenées à 32 niveaux par canal : deux blancs
	// imperceptiblement différents appartiennent au même aplat, et une photo
	// n'en devient pas plate pour autant.
	counts := make(map[uint32]int, 1024)
	total := 0
	for y := b.Min.Y; y < b.Max.Y; y += dominantSample {
		for x := b.Min.X; x < b.Max.X; x += dominantSample {
			r, g, bl, _ := img.At(x, y).RGBA()
			key := uint32(r>>11)<<10 | uint32(g>>11)<<5 | uint32(bl>>11)
			counts[key]++
			total++
		}
	}
	if total == 0 {
		return true
	}

	// Somme des huit teintes les plus présentes, sans trier la carte entière.
	var top [8]int
	for _, n := range counts {
		if n <= top[7] {
			continue
		}
		top[7] = n
		for i := 7; i > 0 && top[i] > top[i-1]; i-- {
			top[i], top[i-1] = top[i-1], top[i]
		}
	}
	dominant := 0
	for _, n := range top {
		dominant += n
	}
	return float64(dominant)/float64(total) < maxDominantShare
}

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

func (h *PagesHandler) isProductPhoto(fileID uint, header []byte) bool {
	if !looksLikePhoto(header) {
		return false
	}
	if verdict, connu := PhotoVerdictCached(fileID); connu {
		return verdict
	}

	var f model.File
	if err := h.db.Select("id, data").First(&f, fileID).Error; err != nil {
		// Illisible : on garde, plutôt que d'écarter la photo d'un producteur
		// sur un incident de lecture.
		RememberPhotoVerdict(fileID, true)
		return true
	}
	verdict := IsPhotograph(f.Data)
	RememberPhotoVerdict(fileID, verdict)
	return verdict
}

const maxDominantShare = 0.50

// dominantSample : un pixel sur trois en largeur comme en hauteur. La
// proportion d'aplats ne se joue pas au pixel près, et l'échantillon divise le
// travail par neuf.
const dominantSample = 3

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

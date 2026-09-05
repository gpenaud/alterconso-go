package handler

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"sync"
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

// maxDominantShare : au-delà, l'image est faite d'aplats — une étiquette, un
// logo, un dessin — et non une photographie.
//
// La mesure sépare les deux familles sans les frôler : sur les images de cette
// installation, huit teintes couvrent 85 % d'une étiquette de fondue et 99 %
// d'un pictogramme, contre 21 % d'une photographie de farine et 6 % d'une
// photographie de poulet. Une photo, fût-elle sur fond clair, garde des
// dégradés que rien ne réduit à huit couleurs.
const maxDominantShare = 0.50

// dominantSample : un pixel sur trois en largeur comme en hauteur. La
// proportion d'aplats ne se joue pas au pixel près, et l'échantillon divise le
// travail par neuf.
const dominantSample = 3

// photoVerdicts retient ce qui a déjà été jugé. Une image de produit ne change
// pratiquement jamais, et la décoder à chaque affichage de l'accueil coûterait
// quelques millisecondes par vignette pour un résultat invariable.
var photoVerdicts sync.Map // map[uint]bool

// PhotoVerdictCached rend le verdict déjà calculé pour ce fichier, s'il existe.
func PhotoVerdictCached(fileID uint) (bool, bool) {
	v, ok := photoVerdicts.Load(fileID)
	if !ok {
		return false, false
	}
	b, _ := v.(bool)
	return b, true
}

// RememberPhotoVerdict garde le verdict d'un fichier.
func RememberPhotoVerdict(fileID uint, isPhoto bool) {
	photoVerdicts.Store(fileID, isPhoto)
}

// IsPhotograph juge une image sur son contenu : est-elle faite de dégradés,
// comme une photographie, ou d'aplats, comme une étiquette ?
//
// Complète looksLikePhoto, qui ne lit que l'en-tête et n'attrape que les
// palettes très pauvres. Une étiquette scannée peut compter cent couleurs tout
// en restant, pour l'essentiel, du blanc et deux encres.
//
// Dans le doute — format inconnu, image illisible — on répond oui : écarter à
// tort la photo d'un producteur est plus dommageable que laisser passer une
// étiquette.
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

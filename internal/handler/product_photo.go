package handler

import "encoding/binary"

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

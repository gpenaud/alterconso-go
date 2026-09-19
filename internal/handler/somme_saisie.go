package handler

import (
	"strconv"
	"strings"
)

// sommeSaisie lit un prix apres pesee, ecrit comme on le compte a voix haute :
// « 4,20+3,15+3,85 ».
//
// POURQUOI ADDITIONNER ICI. Un producteur qui livre trois pieces les pese une a
// une, obtient trois prix, et n'avait jusqu'ici qu'une case pour un seul
// nombre : il posait l'addition de tete, devant l'adherent qui attend. Le
// champ accepte desormais la suite, et c'est le programme qui compte.
//
// L'EVALUATION EST FAITE AUSSI COTE SERVEUR, et pas seulement dans le
// navigateur qui normalise la case au moment ou on la quitte. Valider avec la
// touche Entree n'enleve pas toujours le focus : l'expression partait alors
// telle quelle, « ParseFloat » echouait, et le prix etait ignore SANS UN MOT —
// ni erreur, ni valeur enregistree. Mieux vaut savoir compter des deux cotes
// que perdre une pesee en silence.
//
// Rien d'autre que des nombres, des « + » et des « - » : ce n'est pas un
// evaluateur d'expressions, et il n'a pas a le devenir. Une saisie qui sort de
// la est refusee, l'appelant decide quoi en faire.
//
// La virgule vaut le point : c'est ainsi qu'on ecrit un prix en francais, et
// c'est ce que produit le pave numerique d'un telephone.
func sommeSaisie(brut string) (float64, bool) {
	brut = strings.TrimSpace(brut)
	if brut == "" {
		return 0, false
	}
	brut = strings.ReplaceAll(brut, ",", ".")
	brut = strings.ReplaceAll(brut, " ", "")
	brut = strings.ReplaceAll(brut, " ", "") // espace insecable, frequent au copier-coller

	var total float64
	signe := 1.0
	terme := strings.Builder{}

	ajoute := func() bool {
		if terme.Len() == 0 {
			return false
		}
		v, err := strconv.ParseFloat(terme.String(), 64)
		if err != nil {
			return false
		}
		total += signe * v
		terme.Reset()
		return true
	}

	for i, r := range brut {
		switch {
		case r == '+' || r == '-':
			// Un signe en tete porte sur le premier terme ; ailleurs, il clot
			// celui qu'on vient d'ecrire.
			if i == 0 {
				if r == '-' {
					signe = -1
				}
				continue
			}
			if !ajoute() {
				return 0, false
			}
			signe = 1
			if r == '-' {
				signe = -1
			}
		case (r >= '0' && r <= '9') || r == '.':
			terme.WriteRune(r)
		default:
			return 0, false
		}
	}
	if !ajoute() {
		return 0, false
	}
	return total, true
}

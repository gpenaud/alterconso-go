package handler

import "strings"

// foldForSort met le texte en minuscule et retire les accents pour servir de
// clé de tri : sans ça « Étienne » se classerait après « Zola », les caractères
// accentués passant après l'alphabet non accentué en comparaison d'octets.
func foldForSort(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'à', 'â', 'ä', 'á', 'ã', 'å':
			b.WriteRune('a')
		case 'é', 'è', 'ê', 'ë':
			b.WriteRune('e')
		case 'î', 'ï', 'í', 'ì':
			b.WriteRune('i')
		case 'ô', 'ö', 'ó', 'ò', 'õ':
			b.WriteRune('o')
		case 'ù', 'û', 'ü', 'ú':
			b.WriteRune('u')
		case 'ÿ':
			b.WriteRune('y')
		case 'ç':
			b.WriteRune('c')
		case 'ñ':
			b.WriteRune('n')
		case 'æ':
			b.WriteString("ae")
		case 'œ':
			b.WriteString("oe")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// memberSortKey construit la clé de tri d'un adhérent sur les listes qui
// circulent en distribution (émargement, liste par contrat) : nom de famille
// d'abord, prénom en départage des homonymes. Les adhérents se présentent dans
// un ordre imprévisible, l'ordre alphabétique est le seul qui permette de
// retrouver une ligne du premier coup d'œil.
func memberSortKey(firstName, lastName string) string {
	// Le \x00 évite qu'un nom court suivi d'un prénom déborde sur le nom
	// suivant ("DUR" + "ANDRE" ne doit pas se comparer à "DURAND").
	return foldForSort(lastName) + "\x00" + foldForSort(firstName)
}

package handler

// Déclarations reprises du transcript de la session du 29 août : elles y
// avaient été posées par des scripts visant des numéros de ligne, que le
// rejeu ne pouvait pas reproduire.

// motsVides : ce qui ne distingue pas deux produits.
var motsVides = map[string]bool{
	"de": true, "du": true, "des": true, "la": true, "le": true, "les": true,
	"au": true, "aux": true, "a": true, "à": true, "en": true, "et": true,
	"d": true, "l": true, "sous": true, "vide": true, "bio": true,
}

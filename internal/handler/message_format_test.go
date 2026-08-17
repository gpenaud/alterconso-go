package handler

import (
	"strings"
	"testing"
)

// Le formulaire /messages n'offre qu'un textarea, mais l'email part en HTML :
// sans mise en forme, la saisie arrivait d'un seul bloc chez le destinataire.
func TestMessageBodyKeepsLineBreaks(t *testing.T) {
	out := textToHTML("Bonjour,\nla distribution est avancée.\n\nÀ samedi.")

	if strings.Count(out, "<p ") != 2 {
		t.Errorf("deux paragraphes attendus, obtenu : %s", out)
	}
	if !strings.Contains(out, "Bonjour,<br />la distribution est avancée.") {
		t.Errorf("retour à la ligne simple perdu : %s", out)
	}
}

// Les fins de ligne postées par un textarea varient selon le navigateur ; les
// \r ne doivent jamais ressortir dans le HTML.
func TestMessageBodyNormalizesCRLF(t *testing.T) {
	out := textToHTML("Première ligne\r\nDeuxième ligne\rTroisième ligne")

	if strings.Contains(out, "\r") {
		t.Errorf("retour chariot resté dans le rendu : %q", out)
	}
	if strings.Count(out, "<br />") != 2 {
		t.Errorf("deux sauts attendus, obtenu : %s", out)
	}
}

// Le texte saisi est libre : un « < » ou un « & » ne doit pas casser le rendu
// du message, ni y injecter de balise.
func TestMessageBodyEscapesHTML(t *testing.T) {
	out := textToHTML("Tarifs < 10 € & <script>alert(1)</script>")

	if strings.Contains(out, "<script>") {
		t.Errorf("balise non échappée : %s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("échappement attendu, obtenu : %s", out)
	}
}

// L'expéditeur affiché est le groupe : le message doit le nommer, et rappeler
// qui écrit puisque son nom ne figure plus dans le champ « De ».
func TestMessageHTMLNamesGroupAndSender(t *testing.T) {
	out := renderMessageHTML(
		"Alterconso du Val de Brenne",
		"Guillaume Penaud", "guillaume@example.org",
		"alterconso.leportail.org",
		"Bonjour à tous.",
	)

	for _, want := range []string{
		"Alterconso du Val de Brenne",
		"Guillaume Penaud",
		"mailto:guillaume@example.org",
		"https://alterconso.leportail.org/home",
		"Bonjour à tous.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%q absent du message : %s", want, out)
		}
	}
}

// Un envoi sans groupe ni expéditeur nommé (contact technique, envoi
// automatique) ne doit pas produire de mention vide.
func TestMessageHTMLOmitsMissingParts(t *testing.T) {
	out := renderMessageHTML("", "", "", "", "Message simple.")

	if strings.Contains(out, "Message envoyé par") {
		t.Errorf("signature affichée sans expéditeur : %s", out)
	}
	if strings.Contains(out, "vous êtes inscrit") {
		t.Errorf("mention de groupe affichée sans groupe : %s", out)
	}
	if !strings.Contains(out, "Message simple.") {
		t.Errorf("corps absent : %s", out)
	}
}

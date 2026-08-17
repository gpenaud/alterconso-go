package mailer

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Un sujet français est presque toujours accentué : laissé en UTF-8 brut dans
// l'en-tête, il ressort en caractères illisibles chez une partie des
// destinataires.
func TestHeaderEncodesAccents(t *testing.T) {
	got := encodeHeader("Distribution avancée à jeudi")

	if strings.Contains(got, "é") {
		t.Errorf("accent laissé brut dans l'en-tête : %q", got)
	}
	if !strings.HasPrefix(got, "=?UTF-8?") {
		t.Errorf("encodage RFC 2047 attendu, obtenu : %q", got)
	}
}

// Une valeur purement ASCII n'a rien à encoder : le nom du groupe doit rester
// lisible dans le champ « De ».
func TestHeaderLeavesASCIIAlone(t *testing.T) {
	if got := encodeHeader("Alterconso du Val de Brenne"); got != "Alterconso du Val de Brenne" {
		t.Errorf("valeur ASCII modifiée : %q", got)
	}
}

// Le corps HTML tient sur peu de lignes très longues ; SMTP en refuse au-delà
// de 998 octets.
func TestBodyIsWrappedBase64(t *testing.T) {
	body := "<div>" + strings.Repeat("Bonjour à tous. ", 200) + "</div>"
	got := encodeBody(body)

	for _, line := range strings.Split(strings.TrimRight(got, "\r\n"), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("ligne de %d caractères, 76 au plus attendus", len(line))
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(got, "\r\n", ""))
	if err != nil {
		t.Fatalf("base64 invalide : %v", err)
	}
	if string(decoded) != body {
		t.Error("le corps décodé diffère de l'original")
	}
}

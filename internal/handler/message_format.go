package handler

import (
	"fmt"
	"html/template"
	"strings"
)

// renderMessageHTML met en page le texte saisi dans /messages pour l'envoi.
//
// Le formulaire n'offre qu'un textarea, mais l'email part en `text/html` :
// envoyé tel quel, le message arrivait d'un seul bloc, paragraphes écrasés, et
// le moindre « < » ou « & » saisi cassait le rendu. On échappe donc le texte
// avant de rétablir les retours à la ligne.
//
// L'ensemble est encadré d'un en-tête au nom du groupe et d'un pied qui
// rappelle qui écrit : l'expéditeur affiché par les boîtes de réception est le
// groupe (« Alterconso du Val de Brenne »), pas l'adhérent, dont l'adresse ne
// se trouve que dans le Reply-To.
func renderMessageHTML(groupName, senderName, senderEmail, host, body string) string {
	esc := template.HTMLEscapeString

	var b strings.Builder
	b.WriteString(`<div style="font-family:Arial,Helvetica,sans-serif;font-size:14px;line-height:1.6;color:#333;max-width:640px;margin:0 auto;">`)

	if groupName != "" {
		b.WriteString(fmt.Sprintf(
			`<div style="font-size:17px;font-weight:bold;color:#5a8a00;padding-bottom:10px;margin-bottom:18px;border-bottom:1px solid #e5e5e5;">%s</div>`,
			esc(groupName)))
	}

	b.WriteString(`<div>`)
	b.WriteString(textToHTML(body))
	b.WriteString(`</div>`)

	b.WriteString(`<hr style="border:0;border-top:1px solid #e5e5e5;margin:26px 0 12px;" />`)
	b.WriteString(`<div style="color:#888;font-size:12px;line-height:1.5;">`)
	b.WriteString(messageFooter(groupName, senderName, senderEmail, host))
	b.WriteString(`</div></div>`)

	return b.String()
}

// messageFooter compose le pied de page : qui écrit, depuis quel groupe, et où
// consulter ses commandes. Chaque élément est facultatif — un contact
// technique n'a pas de groupe, un envoi automatique pas d'expéditeur nommé.
func messageFooter(groupName, senderName, senderEmail, host string) string {
	esc := template.HTMLEscapeString
	var parts []string

	switch {
	case senderName != "" && senderEmail != "":
		parts = append(parts, fmt.Sprintf(
			`Message envoyé par %s (<a href="mailto:%s" style="color:#5a8a00;">%s</a>).`,
			esc(senderName), esc(senderEmail), esc(senderEmail)))
	case senderName != "":
		parts = append(parts, fmt.Sprintf("Message envoyé par %s.", esc(senderName)))
	}

	if groupName != "" {
		parts = append(parts, fmt.Sprintf(
			"Vous recevez ce message parce que vous êtes inscrit à <b>%s</b>.", esc(groupName)))
	}

	if host != "" {
		parts = append(parts, fmt.Sprintf(
			`<a href="https://%s/home" style="color:#5a8a00;">Consulter mes commandes</a>`, esc(host)))
	}

	return strings.Join(parts, "<br />")
}

// textToHTML échappe un texte saisi puis restitue sa mise en forme : les lignes
// vides deviennent des paragraphes, les simples retours des <br />. Les trois
// conventions de fin de ligne (\r\n, \r, \n) sont ramenées à \n au préalable :
// selon le navigateur, un textarea poste l'une ou l'autre.
func textToHTML(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var out []string
	for _, para := range strings.Split(s, "\n\n") {
		para = strings.Trim(para, "\n")
		if strings.TrimSpace(para) == "" {
			continue
		}
		lines := strings.Split(para, "\n")
		for i, l := range lines {
			lines[i] = template.HTMLEscapeString(l)
		}
		out = append(out, fmt.Sprintf(`<p style="margin:0 0 14px;">%s</p>`,
			strings.Join(lines, "<br />")))
	}

	return strings.Join(out, "")
}

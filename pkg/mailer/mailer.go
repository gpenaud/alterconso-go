package mailer

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/smtp"
	"strings"

	"github.com/gpenaud/alterconso/internal/config"
)

// encodeHeader rend un en-tête transportable : les valeurs purement ASCII
// passent telles quelles, les autres sont encodées en Q (RFC 2047).
func encodeHeader(s string) string {
	return mime.QEncoding.Encode("UTF-8", s)
}

// encodeBody encode le corps en base64 découpé en lignes de 76 caractères,
// comme l'impose le format MIME.
func encodeBody(body string) string {
	enc := base64.StdEncoding.EncodeToString([]byte(body))

	var b strings.Builder
	for len(enc) > 76 {
		b.WriteString(enc[:76])
		b.WriteString("\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc)
	b.WriteString("\r\n")
	return b.String()
}

// Mail représente un email à envoyer.
type Mail struct {
	From     string
	FromName string
	To       []Recipient
	ReplyTo  string
	Subject  string
	HTMLBody string
}

type Recipient struct {
	Email string
	Name  string
}

func (m *Mail) AddRecipient(email, name string) {
	m.To = append(m.To, Recipient{Email: email, Name: name})
}

// Mailer envoie des emails via SMTP.
type Mailer struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Mailer {
	return &Mailer{cfg: cfg}
}

// Send envoie un email. En mode debug, affiche dans les logs sans envoyer.
func (m *Mailer) Send(mail *Mail) error {
	if m.cfg.Debug {
		// En développement : log uniquement
		fmt.Printf("[MAIL DEBUG] To: %v | Subject: %s\n", mail.To, mail.Subject)
		return nil
	}
	return m.sendSMTP(mail)
}

func (m *Mailer) sendSMTP(mail *Mail) error {
	addr := fmt.Sprintf("%s:%s", m.cfg.SMTPHost, m.cfg.SMTPPort)
	fmt.Printf("[MAIL] sending to %v via %s user=%s\n", mail.To, addr, m.cfg.SMTPUser)

	fromAddr := mail.From
	if fromAddr == "" {
		fromAddr = m.cfg.DefaultEmail
	}

	toAddrs := make([]string, len(mail.To))
	toHeaders := make([]string, len(mail.To))
	for i, r := range mail.To {
		toAddrs[i] = r.Email
		if r.Name != "" {
			toHeaders[i] = fmt.Sprintf("%s <%s>", encodeHeader(r.Name), r.Email)
		} else {
			toHeaders[i] = r.Email
		}
	}

	fromHeader := fromAddr
	if mail.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", encodeHeader(mail.FromName), fromAddr)
	}

	headers := []string{
		fmt.Sprintf("From: %s", fromHeader),
		fmt.Sprintf("To: %s", strings.Join(toHeaders, ", ")),
		// Un sujet français est presque toujours accentué : sans encodage
		// RFC 2047, l'UTF-8 brut dans l'en-tête ressort en caractères
		// illisibles selon la messagerie du destinataire.
		fmt.Sprintf("Subject: %s", encodeHeader(mail.Subject)),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		// Le corps HTML tient sur peu de lignes très longues, au-delà de la
		// limite SMTP de 998 octets : le base64 le redécoupe et met du même
		// coup les accents à l'abri des relais 7 bits.
		"Content-Transfer-Encoding: base64",
	}
	if mail.ReplyTo != "" {
		headers = append(headers, fmt.Sprintf("Reply-To: %s", mail.ReplyTo))
	}

	msg := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + encodeBody(mail.HTMLBody))

	// Brevo port 587 = STARTTLS
	auth := smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPassword, m.cfg.SMTPHost)
	err := smtp.SendMail(addr, auth, fromAddr, toAddrs, msg)
	if err != nil {
		fmt.Printf("[MAIL] ERROR: %v\n", err)
	} else {
		fmt.Printf("[MAIL] sent OK\n")
	}
	return err
}

// QuickMail envoie un email simple texte/HTML.
func (m *Mailer) QuickMail(to, subject, html string) error {
	mail := &Mail{
		From:     m.cfg.DefaultEmail,
		FromName: "Alterconso",
		Subject:  subject,
		HTMLBody: html,
	}
	mail.AddRecipient(to, "")
	return m.Send(mail)
}

package mailer

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/gpenaud/alterconso/internal/config"
)

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
			toHeaders[i] = fmt.Sprintf("%s <%s>", r.Name, r.Email)
		} else {
			toHeaders[i] = r.Email
		}
	}

	fromHeader := fromAddr
	if mail.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", mail.FromName, fromAddr)
	}

	headers := []string{
		fmt.Sprintf("From: %s", fromHeader),
		fmt.Sprintf("To: %s", strings.Join(toHeaders, ", ")),
		fmt.Sprintf("Subject: %s", mail.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
	}
	if mail.ReplyTo != "" {
		headers = append(headers, fmt.Sprintf("Reply-To: %s", mail.ReplyTo))
	}

	msg := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + mail.HTMLBody)

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

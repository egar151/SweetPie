// Package notify provides email notification functionality.
package notify

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"text/template"
	"time"

	"sftp-sync/internal/config"
	"sftp-sync/pkg/logger"
)

// EmailSender sends email notifications via SMTP.
type EmailSender struct {
	cfg    config.NotificationConfig
	logger *logger.Logger
}

// NewEmailSender creates a new EmailSender.
func NewEmailSender(cfg config.NotificationConfig, log *logger.Logger) *EmailSender {
	return &EmailSender{
		cfg:    cfg,
		logger: log.WithComponent("email"),
	}
}

// Send sends an email with the given subject and body.
func (s *EmailSender) Send(subject, body string) error {
	if !s.cfg.Enabled {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	// Build message
	msg := s.buildMessage(subject, body)

	var err error
	if s.cfg.SkipTLSVerify {
		err = s.sendMailInsecure(addr, s.cfg.FromAddress, s.cfg.ToAddresses, []byte(msg))
	} else {
		err = smtp.SendMail(addr, nil, s.cfg.FromAddress, s.cfg.ToAddresses, []byte(msg))
	}

	if err != nil {
		s.logger.Error().
			Err(err).
			Str("subject", subject).
			Msg("Failed to send email")
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info().
		Str("subject", subject).
		Int("recipients", len(s.cfg.ToAddresses)).
		Msg("Email sent")

	return nil
}

// sendMailInsecure sends email with TLS certificate verification disabled.
func (s *EmailSender) sendMailInsecure(addr, from string, to []string, msg []byte) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.cfg.SMTPHost)
	if err != nil {
		return fmt.Errorf("smtp client creation failed: %w", err)
	}
	defer client.Close()

	// Check if STARTTLS is supported
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName:         s.cfg.SMTPHost,
			InsecureSkipVerify: true,
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls failed: %w", err)
		}
	}

	if err = client.Mail(from); err != nil {
		return fmt.Errorf("mail from failed: %w", err)
	}

	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("rcpt to failed: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data command failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write data failed: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("close data failed: %w", err)
	}

	return client.Quit()
}

// buildMessage constructs the email message.
func (s *EmailSender) buildMessage(subject, body string) string {
	var msg bytes.Buffer

	msg.WriteString(fmt.Sprintf("From: %s\r\n", s.cfg.FromAddress))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(s.cfg.ToAddresses, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.String()
}

// EmailTemplate represents an email template.
type EmailTemplate struct {
	Subject string
	Body    string
}

// Templates for different notification types.
var templates = map[string]EmailTemplate{
	config.NotifyConnectionFailure: {
		Subject: "[SFTP Sync] Connection Failure: {{.Connection}}",
		Body: `SFTP Sync Service - Connection Failure Alert

Connection: {{.Connection}}
Host: {{.Host}}
Error: {{.Error}}
Time: {{.Time}}

Please check the SFTP server connection and credentials.

---
This is an automated message from SFTP Sync Service.`,
	},
	config.NotifyTransferFailure: {
		Subject: "[SFTP Sync] Transfer Failure: {{.FileName}}",
		Body: `SFTP Sync Service - Transfer Failure Alert

File: {{.FileName}}
Rule: {{.Rule}}
Direction: {{.Direction}}
Error: {{.Error}}
Time: {{.Time}}

The file transfer failed after {{.Attempts}} attempt(s).

---
This is an automated message from SFTP Sync Service.`,
	},
	config.NotifyTransferCompleted: {
		Subject: "[SFTP Sync] Transfer Completed: {{.Rule}}",
		Body: `SFTP Sync Service - Transfer Completed

Rule: {{.Rule}}
Direction: {{.Direction}}
Files Transferred: {{.FileCount}}
Time: {{.Time}}

Files:
{{.FileList}}

---
This is an automated message from SFTP Sync Service.`,
	},
	config.NotifyServiceStart: {
		Subject: "[SFTP Sync] Service Started",
		Body: `SFTP Sync Service - Service Started

The SFTP Sync Service has started successfully.

Version: {{.Version}}
Time: {{.Time}}
Rules: {{.RuleCount}}
Connections: {{.ConnectionCount}}

---
This is an automated message from SFTP Sync Service.`,
	},
	config.NotifyServiceStop: {
		Subject: "[SFTP Sync] Service Stopped",
		Body: `SFTP Sync Service - Service Stopped

The SFTP Sync Service has stopped.

Time: {{.Time}}
Reason: {{.Reason}}

---
This is an automated message from SFTP Sync Service.`,
	},
}

// SendTemplated sends an email using a predefined template.
func (s *EmailSender) SendTemplated(templateName string, data map[string]interface{}) error {
	tmpl, ok := templates[templateName]
	if !ok {
		return fmt.Errorf("unknown template: %s", templateName)
	}

	// Add current time if not provided
	if _, ok := data["Time"]; !ok {
		data["Time"] = time.Now().Format(time.RFC3339)
	}

	// Parse and execute subject template
	subjectTmpl, err := template.New("subject").Parse(tmpl.Subject)
	if err != nil {
		return fmt.Errorf("failed to parse subject template: %w", err)
	}

	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, data); err != nil {
		return fmt.Errorf("failed to execute subject template: %w", err)
	}

	// Parse and execute body template
	bodyTmpl, err := template.New("body").Parse(tmpl.Body)
	if err != nil {
		return fmt.Errorf("failed to parse body template: %w", err)
	}

	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
		return fmt.Errorf("failed to execute body template: %w", err)
	}

	return s.Send(subjectBuf.String(), bodyBuf.String())
}

// ShouldNotify checks if the given event type is configured for notifications.
func (s *EmailSender) ShouldNotify(eventType string) bool {
	if !s.cfg.Enabled {
		return false
	}

	for _, e := range s.cfg.NotifyOn {
		if e == eventType {
			return true
		}
	}

	return false
}

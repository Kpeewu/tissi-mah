package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SMTPProvider envoie des emails via SMTP avec SSL/TLS (port 465).
// Conçu pour les serveurs OVH (ssl0.ovh.net) mais compatible avec tout serveur SMTP SSL.
type SMTPProvider struct {
	host     string
	port     string
	username string
	password string
	from     string
	logger   *zap.Logger
}

// NewSMTPProvider crée un nouveau provider SMTP.
// Le FROM doit être identique au username pour que la signature DKIM OVH s'applique.
func NewSMTPProvider(host, port, username, password, from string, logger *zap.Logger) *SMTPProvider {
	return &SMTPProvider{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		logger:   logger,
	}
}

func (p *SMTPProvider) Send(ctx context.Context, to, subject, bodyText, bodyHTML string) (string, error) {
	p.logger.Debug("smtp: sending email",
		zap.String("to", to),
		zap.String("subject", subject),
		zap.String("host", p.host),
	)

	body, err := buildMIMEBody(p.from, to, subject, bodyText, bodyHTML)
	if err != nil {
		return "", fmt.Errorf("smtp: build mime body: %w", err)
	}

	addr := p.host + ":" + p.port
	tlsConfig := &tls.Config{
		ServerName: p.host,
		MinVersion: tls.VersionTLS12,
	}

	// Connexion SSL/TLS directe (port 465) — pas de STARTTLS
	tlsDialer := &tls.Dialer{
		NetDialer: &net.Dialer{},
		Config:    tlsConfig,
	}
	rawConn, err := tlsDialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		p.logger.Error("smtp: TLS dial failed", zap.String("addr", addr), zap.Error(err))
		return "", fmt.Errorf("smtp: dial %s: %w", addr, err)
	}
	conn := rawConn.(*tls.Conn)
	defer conn.Close() //nolint:errcheck

	client, err := smtp.NewClient(conn, p.host)
	if err != nil {
		return "", fmt.Errorf("smtp: new client: %w", err)
	}
	defer client.Quit() //nolint:errcheck

	auth := smtp.PlainAuth("", p.username, p.password, p.host)
	if err := client.Auth(auth); err != nil {
		p.logger.Error("smtp: authentication failed", zap.String("username", p.username), zap.Error(err))
		return "", fmt.Errorf("smtp: auth: %w", err)
	}

	if err := client.Mail(p.from); err != nil {
		return "", fmt.Errorf("smtp: MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return "", fmt.Errorf("smtp: RCPT TO: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("smtp: DATA: %w", err)
	}
	if _, err := wc.Write(body); err != nil {
		wc.Close() //nolint:errcheck
		return "", fmt.Errorf("smtp: write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("smtp: close data writer: %w", err)
	}

	messageID := fmt.Sprintf("<%s@tissimah>", uuid.NewString())
	p.logger.Info("smtp: email sent",
		zap.String("to", to),
		zap.String("messageId", messageID),
	)
	return messageID, nil
}

func (p *SMTPProvider) Name() string {
	return "smtp_ovh"
}

// buildMIMEBody construit un message MIME multipart/alternative (text + html).
func buildMIMEBody(from, to, subject, bodyText, bodyHTML string) ([]byte, error) {
	var buf bytes.Buffer

	// En-têtes principaux
	buf.WriteString("From: TissiMah <" + from + ">\r\n")
	buf.WriteString("To: " + to + "\r\n")
	buf.WriteString("Subject: " + subject + "\r\n")
	buf.WriteString("Date: " + time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000") + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")

	mw := multipart.NewWriter(&buf)
	buf.WriteString("Content-Type: multipart/alternative; boundary=\"" + mw.Boundary() + "\"\r\n")
	buf.WriteString("\r\n")

	// Partie text/plain
	plainHeader := make(textproto.MIMEHeader)
	plainHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	plainHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	pw, err := mw.CreatePart(plainHeader)
	if err != nil {
		return nil, err
	}
	if _, err := pw.Write([]byte(bodyText)); err != nil {
		return nil, err
	}

	// Partie text/html
	htmlHeader := make(textproto.MIMEHeader)
	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	hw, err := mw.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}
	if _, err := hw.Write([]byte(bodyHTML)); err != nil {
		return nil, err
	}

	if err := mw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

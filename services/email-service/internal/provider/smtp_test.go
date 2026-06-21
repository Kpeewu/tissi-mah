package provider

import (
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

// TestBuildMIMEBody_LinkSurvivesQuotedPrintable garantit qu'un lien contenant un
// `=` (ex: `?token=…`) est correctement encodé en quoted-printable puis décodé
// sans corruption — régression du bug « le lien n'apparaît pas dans le mail ».
func TestBuildMIMEBody_LinkSurvivesQuotedPrintable(t *testing.T) {
	link := "https://support.test/reset-password?token=AbC_-123xyz"
	text := "Ouvrez ce lien : " + link
	html := `<a href="` + link + `">Réinitialiser</a>`

	raw, err := buildMIMEBody("noreply@x.com", "to@x.com", "Réinitialisation", text, html)
	if err != nil {
		t.Fatalf("buildMIMEBody: %v", err)
	}
	msg := string(raw)

	// Le `=` doit être encodé en `=3D` sur le fil (preuve que la QP est appliquée).
	if !strings.Contains(msg, "token=3DAbC_-123xyz") {
		t.Fatalf("le `=` du lien n'est pas encodé en quoted-printable:\n%s", msg)
	}

	// Décodage de chaque partie : le lien original doit réapparaître intact.
	// NB : multipart.Part décode automatiquement le quoted-printable d'après
	// l'entête Content-Transfer-Encoding — pas besoin de re-décoder.
	boundary := extractBoundary(t, msg)
	mr := multipart.NewReader(strings.NewReader(bodyAfterHeaders(msg)), boundary)
	parts := 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		decoded, err := io.ReadAll(p)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		if !strings.Contains(string(decoded), link) {
			t.Fatalf("lien corrompu après décodage:\n%s", string(decoded))
		}
		parts++
	}
	if parts != 2 {
		t.Fatalf("attendu 2 parties (text+html), obtenu %d", parts)
	}
}

func TestBuildMIMEBody_SubjectEncoded(t *testing.T) {
	raw, err := buildMIMEBody("noreply@x.com", "to@x.com", "Réinitialisation", "x", "x")
	if err != nil {
		t.Fatalf("buildMIMEBody: %v", err)
	}
	// Le sujet accentué doit être encodé en encoded-word et décoder à l'original.
	for _, line := range strings.Split(string(raw), "\r\n") {
		if strings.HasPrefix(line, "Subject: ") {
			dec, err := new(mime.WordDecoder).DecodeHeader(strings.TrimPrefix(line, "Subject: "))
			if err != nil {
				t.Fatalf("decode subject: %v", err)
			}
			if dec != "Réinitialisation" {
				t.Fatalf("sujet décodé inattendu: %q", dec)
			}
			return
		}
	}
	t.Fatal("entête Subject absente")
}

func extractBoundary(t *testing.T, msg string) string {
	t.Helper()
	const marker = `boundary="`
	i := strings.Index(msg, marker)
	if i < 0 {
		t.Fatal("boundary absente")
	}
	rest := msg[i+len(marker):]
	return rest[:strings.IndexByte(rest, '"')]
}

func bodyAfterHeaders(msg string) string {
	if i := strings.Index(msg, "\r\n\r\n"); i >= 0 {
		return msg[i+4:]
	}
	return msg
}

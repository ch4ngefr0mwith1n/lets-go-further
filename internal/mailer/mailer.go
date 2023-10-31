package mailer

import (
	"bytes"
	"embed"
	"html/template"
	"time"

	"github.com/go-mail/mail/v2"
)

// ova promjenjiva je "embed.FS" tipa (embedded file system)
// unutar nje će se nalaziti "email" templejti
// sav sadržaj "./templates" direktorijuma će automatski biti učitan u ovu promjenjivu
// ↓↓↓

//go:embed "templates"
var templateFS embed.FS

type Mailer struct {
	// "mail.Dialer" instanca služi za povezivanje sa SMTP serverom
	dialer *mail.Dialer
	// informacije o pošiljaocu (recimo, "support@mrkic.com")
	sender string
}

func New(host string, port int, username, password, sender string) Mailer {
	// inicijalizuje se "mail.Dialer()" instanca sa zadatim SMTP podešavanjima
	dialer := mail.NewDialer(host, port, username, password)
	// koristiće se "timeout" od 5 sekundi svaki put kada pošaljemo e-mail
	dialer.Timeout = 5 * time.Second

	return Mailer{
		dialer: dialer,
		sender: sender,
	}
}

func (m Mailer) Send(recipient string, templateFile string, data any) error {
	tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}

	// "subject" templejt će biti izvršen, skupa sa dinamičkim podacima iz parametra funkcije
	// rezultat će se sačuvati u "subject" promjenjivoj
	// isti šablon ćemo koristiti i za dva preostala templejta
	subject := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}

	plainBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err
	}

	htmlBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}

	// inicijalizovanje nove "mail.Message()" instance
	msg := mail.NewMessage()

	msg.SetHeader("To", recipient)
	msg.SetHeader("From", m.sender)
	msg.SetHeader("Subject", subject.String())

	// "body" treba da bude u "text/plain" formatu:
	msg.SetBody("text/plain", plainBody.String())
	msg.AddAlternative("text/html", htmlBody.String())

	// "DialAndSend()" metoda otvara konekciju ka SMTP serveru, šalje poruku i nakon toga zatvara konekciju
	// ukoliko istekne "timeout", onda će vratiti "dial tcp: i/o timeout" grešku
	// postojaće tri pokušaja da se pošalje mejl prije odustajanja i slanja zadnje greške
	// biće čekanje od 500 milisekundi između pokušaja
	for i := 1; i <= 3; i++ {
		err = m.dialer.DialAndSend(msg)
		// If everything worked, return nil.
		if nil == err {
			return nil
		}

		// If it didn't work, sleep for a short time and retry.
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

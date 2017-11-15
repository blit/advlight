package tickets

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/matcornic/hermes"
	gomail "gopkg.in/gomail.v2"
)

var mailer *hermes.Hermes
var smtpConfig *smtpconfig

func init() {
	mailer = &hermes.Hermes{
		// Optional Theme
		Theme: new(hermes.Flat),
		Product: hermes.Product{
			// Appears in header & footer of e-mails
			Name: "Bayside Christmas Adventure -- Christmas Lights Drive-Thru",
			Link: "http://christmas.baysideonline.com/lights",
			// Optional product logo
			Logo:      "http://v.fastcdn.co/t/c179d187/b85e76d2/1510270294-24260441-767x176x960x540x68x176-Christmas-Website---.png",
			Copyright: "Sent with Love from your friends at Bayside Church",
		},
	}
	smtpConfig = &smtpconfig{} // empty config will log emails (useful for dev)
	if os.Getenv("ADVLIGHT_SMTP") != "" {
		err := smtpConfig.Parse(os.Getenv("ADVLIGHT_SMTP"))
		if err != nil {
			log.Panicf("invalid SMTP config(%v): %s ", err, os.Getenv("ADVLIGHT_SMTP"))
		}
	}
}

func ConfirmationEmail(g Guest) hermes.Email {
	return hermes.Email{
		Body: hermes.Body{
			Name: g.Email,
			Intros: []string{
				"You have received this email to confirm your ticket for a Bayside Christmas Adventure -- Christmas Lights Drive-Thru",
			},
			Actions: []hermes.Action{
				{
					Instructions: "Click the button below to confirm/view your ticket:",
					Button: hermes.Button{
						Color: "#0F8A5F",
						Text:  "Confirm",
						Link:  HostName + "/" + g.GetToken(),
					},
				},
				{
					//					Instructions: "Donations:",
					Button: hermes.Button{
						Color: "#235E6F",
						Text:  "Donate",
						Link:  "http://granitebay.baysideonline.com/christmas-drive-thru-giving/",
					},
				},
			},
			Outros: []string{
				"If you did not request this reservation no further action is required on your part and you will not be sent further emails or added to an email list.",
			},
			Signature: "Merry Christmas!",
		},
	}
}

func SendEmail(address string, email hermes.Email) error {
	// Generate an HTML email with the provided contents (for modern clients)
	htmlpart, err := mailer.GenerateHTML(email)
	if err != nil {
		return err
	}
	// Generate the plaintext version of the e-mail (for clients that do not support xHTML)
	textpart, err := mailer.GeneratePlainText(email)
	if err != nil {
		return err
	}
	m := gomail.NewMessage()
	m.SetHeader("From", `"Bayside Christmas Lights" <support@blit.com>`)
	m.SetHeader("To", address)
	m.SetHeader("Subject", "Confirm your Bayside Christmas Drive Through Tickets")
	m.SetBody("text/plain", textpart)
	m.AddAlternative("text/html", htmlpart)

	log.Println("sending email to ", m.GetHeader("To"))
	if smtpConfig.Hostname == "" {
		m.WriteTo(os.Stdout)
		return nil
	}

	d := gomail.NewDialer(smtpConfig.Hostname, smtpConfig.Port, smtpConfig.Username, smtpConfig.Password)
	s, err := d.Dial()
	if err != nil {
		return err
	}
	err = gomail.Send(s, m)
	if err != nil {
		return err
	}
	return nil
}

type smtpconfig struct {
	Hostname, Username, Password string
	Port                         int
}

func (c *smtpconfig) Parse(s string) error {
	parts := strings.Split(s, ":")
	if len(parts) != 4 {
		return fmt.Errorf("smtpconfig.Parse requires arity 4")
	}
	for idx, part := range parts {
		switch idx {
		case 0:
			c.Username = part
		case 1:
			c.Password = part
		case 2:
			c.Hostname = part
		case 3:
			p, err := strconv.Atoi(part)
			if err != nil {
				return err
			}
			c.Port = p
		}
	}
	return nil
}

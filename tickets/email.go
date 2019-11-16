package tickets

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blit/advlight/config"
	"github.com/matcornic/hermes"
	gomail "gopkg.in/gomail.v2"
)

var mailer *hermes.Hermes
var smtpConfig *smtpconfig
var Mailer *mailerHelper

func init() {
	Mailer = &mailerHelper{}
	mailer = &hermes.Hermes{
		// Optional Theme
		Theme: new(hermes.Flat),
		Product: hermes.Product{
			// Appears in header & footer of e-mails
			Name: config.EventName,
			Link: config.EventLink,
			// Optional product logo
			Logo:      config.EventLogo,
			Copyright: "Sent with Love from your friends at " + config.ChurchName,
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

func ConfirmationEmail(g Guest, slot time.Time) hermes.Email {
	actions := []hermes.Action{
		{
			Instructions: "Click the button below to confirm/view your ticket:",
			Button: hermes.Button{
				Color: "#4CAF50",
				Text:  "Confirm | View Ticket",
				Link:  g.GetTicketURL(slot),
			},
		},
	}
	if config.DonateLink != "" {
		actions = append(actions, hermes.Action{
			Button: hermes.Button{
				Color: "#2196F3",
				Text:  "Donate",
				Link:  config.DonateLink,
			},
		})
	}
	return hermes.Email{
		Body: hermes.Body{
			Name: g.Email,
			Intros: []string{
				"You have received this email to confirm your ticket for " + config.EventName,
			},
			Actions: actions,
			Outros: []string{
				"If you did not request this reservation no further action is required on your part and you will not be sent further emails or added to an email list.",
			},
			Signature: "Merry Christmas!",
		},
	}
}

func ExpirationEmail(g Guest, slot time.Time) hermes.Email {
	return hermes.Email{
		Body: hermes.Body{
			Name: g.Email,
			Intros: []string{
				fmt.Sprintf("Your %s ticket request for %s has expired.  If you would still like a ticket, use the link below to select a ticket and then be sure to click the confirmation link sent to you.  If you do not click the confirmation link, your ticket will expire.", config.EventName, slot.Format("Jan 02, 3:04pm")),
			},
			Actions: []hermes.Action{
				{
					Instructions: "To get another ticket, or to view your tickets:",
					Button: hermes.Button{
						Color: "#0F8A5F",
						Text:  "Get | View Tickets",
						Link:  g.GetGuestURL(),
					},
				},
			},
			Outros: []string{
				"If you did not request this ticket no further action is required on your part and you will not be sent further emails or added to an email list.",
			},
			Signature: "Merry Christmas!",
		},
	}
}

type mailerHelper struct {
	sync   sync.Mutex
	dialer *gomail.Dialer
	sender gomail.SendCloser
}

func (m *mailerHelper) Send(address, subject string, email hermes.Email) error {
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
	msg := gomail.NewMessage()
	msg.SetHeader("From", `"`+config.EventName+`" <support@blit.com>`)
	msg.SetHeader("To", address)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/plain", textpart)
	msg.AddAlternative("text/html", htmlpart)

	log.Println("sending email to ", msg.GetHeader("To"))
	if smtpConfig.Hostname == "" {
		msg.WriteTo(os.Stdout)
		return nil
	}

	m.sync.Lock()
	// defer m.sync.Unlock()
	// cant defer here because we recurse once on error and can get a panic sync: unlock of unlocked mutex
	if m.dialer == nil {
		m.dialer = gomail.NewDialer(smtpConfig.Hostname, smtpConfig.Port, smtpConfig.Username, smtpConfig.Password)
	}
	tryAgain := false
	if m.sender == nil {
		m.sender, err = m.dialer.Dial()
		if err != nil {
			m.sender = nil
			m.sync.Unlock()
			return err
		}
	} else {
		tryAgain = true // retry the email if the sender has been closed
	}

	err = gomail.Send(m.sender, msg)
	if err != nil && tryAgain {
		m.sender.Close()
		m.sender = nil
		m.sync.Unlock()
		return m.Send(address, subject, email)
	}
	m.sync.Unlock()
	return err
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

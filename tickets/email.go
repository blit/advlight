package tickets

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
			Name: "Bayside Christmas Lights Drive-Thru",
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

func ConfirmationEmail(g Guest, slot time.Time) hermes.Email {
	return hermes.Email{
		Body: hermes.Body{
			Name: g.Email,
			Intros: []string{
				"You have received this email to confirm your ticket for the Bayside Christmas Lights Drive-Thru",
			},
			Actions: []hermes.Action{
				{
					Instructions: "Click the button below to confirm/view your ticket:",
					Button: hermes.Button{
						Color: "#0F8A5F",
						Text:  "Confirm | View Ticket",
						Link:  g.GetTicketURL(slot),
					},
				},
				{
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

func ExpirationEmail(g Guest, slot time.Time) hermes.Email {
	return hermes.Email{
		Body: hermes.Body{
			Name: g.Email,
			Intros: []string{
				fmt.Sprintf("Your Bayside Christmas Drive-Thru ticket request for %s has expired.  If you would still like a ticket, use the link below to select a ticket and then be sure to click the confirmation link sent to you.  If you do not click the confirmation link, your ticket will expire.", slot.Format("Jan 02, 3:04pm")),
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

func GraceEmail(g Guest) hermes.Email {
	return hermes.Email{
		Body: hermes.Body{
			Name: g.Email,
			Intros: strings.Split(`
				Merry Christmas from Bayside!
				
			 We have a first for Bayside... our Special Christmas Event at our Granite Bay Campus sold out in less then three days. While we’re grateful, this means thousands of families are currently unable to come to a Christmas Eve Service.
				
			 Our solution has been to move our Granite Bay Christmas Experience to our Adventure Campus which will add 10,000+ new seats for families across the region. Unfortunately, this means we need to close our Drive Thru on the following dates:
				
			 December 15, 19, 20,21 and 23.
				
			 Because you have reserved a ticket on one of those dates, we hope you understand and join us on another night. We’ve added tickets on almost every other night to accommodate and will give you priority to those. We know this can be an inconvenience so we want to help make changing your time a little easier. 

			 Your existing ticket(s) for December 15, 19, 20,21 and 23 will remain in the system for a few days and then be deleted (please cancel it once you've selected a different ticket).
				
			 To change your reservation to another night, use the buttons below to pick another ticket.
				
			`, "\n"),
			Actions: []hermes.Action{
				{
					Instructions: "GRACE event code reserved tickets (added exclusively for people that had a ticket for December 15, 19, 20, 21 and 23).",
					Button: hermes.Button{
						Color: "#0F8A5F",
						Text:  "Get | GRACE Tickets",
						Link:  g.GetGuestURL() + "?event=grace",
					},
				},
				{
					Instructions: "All available general admission tickets:",
					Button: hermes.Button{
						Color: "#0F8A5F",
						Text:  "Get | General Tickets",
						Link:  g.GetGuestURL(),
					},
				},
			},
			Outros: []string{
				"Thank you for patience and understanding.",
			},
			Signature: "Merry Christmas!",
		},
	}
}

func GraceFixEmail(g Guest) hermes.Email {
	return hermes.Email{
		Body: hermes.Body{
			Name: g.Email,
			Intros: strings.Split(`
			Merry Christmas from Bayside!
				
			 Please disregard the last email advising your Christmas Drive Thru Ticket has been cancelled.
				
			 Your ticket is for the 22nd and is not affected by the closure on December 15, 19, 20,21 and 23.
				
			 Sorry for the confusion.
				
			`, "\n"),
			Actions: []hermes.Action{
				{
					Instructions: "To view your ticket, or change the time, use the button below:",
					Button: hermes.Button{
						Color: "#0F8A5F",
						Text:  "Get | View Tickets",
						Link:  g.GetGuestURL(),
					},
				},
			},
			Outros: []string{
				"Thank you for patience and understanding.",
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
	msg.SetHeader("From", `"Bayside Christmas Lights" <support@blit.com>`)
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

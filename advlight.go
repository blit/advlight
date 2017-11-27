package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/blit/advlight/tickets"

	"github.com/blit/advlight/views"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/lib/pq" // required for database/sql
)

func main() {
	cmdPtr := flag.String("cmd", "server", "-cmd: server(default) || expired")
	flag.Parse()
	switch *cmdPtr {
	case "expired":
		fmt.Println("running expired tickets cmd")
		guests, err := tickets.Repo.GetExpiredGuests("1 hour")
		if err != nil {
			panic(err)
		}
		fmt.Printf("notifying %d\n guests of expiration", len(guests))
		for _, g := range guests {
			slot := g.Tickets[0]
			em := tickets.ExpirationEmail(*g, slot.Slot)
			subject := fmt.Sprintf("Your Bayside Christmas Drive-Thru ticket request expired (%s)", slot.Slot.Format("Jan 02, 3:04pm"))
			err = tickets.Mailer.Send(g.Email, subject, em)
			if err != nil {
				fmt.Println("ERROR", g.Email, err)
			}
			err = tickets.Repo.CancelTicket(g, slot.Slot)
			if err != nil {
				fmt.Println("DB-ERROR", g.Email, err)
			}
		}
		return
	default:
		runServer()
	}
	return
}

func runServer() {
	r := chi.NewRouter()
	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/", views.TicketIndexHandler)
	r.Post("/", views.TicketIndexHandler)

	r.Get("/admin", views.TicketAdminHandler)
	r.Post("/admin", views.TicketAdminHandler)

	r.Get("/{guestID}", views.TicketIndexHandler)
	r.Post("/{guestID}", views.TicketIndexHandler)
	r.Get("/{guestID}/ticket/{ticketID}", views.TicketShowHandler)
	r.Get("/assets/img/{imageID}", views.AssetImageHandler)
	if tickets.DatabaseURL == "" {
		log.Println(os.Getenv("ADVLIGHT_DATABASE_URL"))
		log.Fatal("ADVLIGHT_DATABASE_URL is not set; try export ADVLIGHT_DATABASE_URL=postgres://postgres@localhost/advlight?sslmode=disable")
	}
	log.Println(tickets.HostName, "\n", tickets.DatabaseURL)
	http.ListenAndServe(":8080", r)
}

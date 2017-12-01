package main

import (
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

	r.Get("/admin/run_expired", views.TicketAdminExpiresHandler)

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

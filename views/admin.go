package views

import (
	"log"
	"net/http"
	"os"

	"github.com/blit/advlight/tickets"
)

func TicketAdminHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	data := struct {
		ErrorMsg       string
		Stats          []tickets.SlotStat
		TotalTickets   int64
		TotalBooked    int64
		TotalAvailable int64
		Password       string
	}{
		"",  // ErrorMsg
		nil, // Stats
		0,   // TotalTickets
		0,   // TotalBooked
		9,   // TotalAvailable
		os.Getenv("ADVLIGHT_PASSWORD"),
	}

	if r.Method == "POST" {
		if r.FormValue("password") == data.Password {
			data.Stats, err = tickets.Repo.GetSlotsStats()
			if err != nil {
				data.ErrorMsg = err.Error()
			} else {
				// tally counts
				for _, s := range data.Stats {
					data.TotalTickets += s.NumberTickets
					data.TotalBooked += (s.NumberTickets - s.AvailableTickets)
					data.TotalAvailable += s.AvailableTickets
				}
			}
		} else {
			data.ErrorMsg = "Invalid password"
		}

	}

	log.Println("TicketAdminHandler", data.ErrorMsg)
	Render(w, "admin.html", data)
	return
}

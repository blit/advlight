package views

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/blit/advlight/config"
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
		AddTickets     string
	}{
		"",                             // ErrorMsg
		nil,                            // Stats
		0,                              // TotalTickets
		0,                              // TotalBooked
		9,                              // TotalAvailable
		os.Getenv("ADVLIGHT_PASSWORD"), // Password
		"",                             // AddTickets
	}

	if r.Method == "POST" {

		if r.FormValue("download") == "true" && r.FormValue("password") == data.Password {
			w.Header().Set("Content-Disposition", "attachment; filename=guests.csv")
			w.Header().Set("Content-Type", "text/csv")
			tickets.Repo.ToCSV(w)
			return
		}

		if r.FormValue("addTickets") != "" && r.FormValue("password") == data.Password {
			var addSlot, addCount int
			// add value will be addSlot+AddCount
			parts := strings.Split(r.FormValue("addTickets"), "+")
			if len(parts) == 2 {
				addSlot, _ = strconv.Atoi(parts[0])
				addCount, _ = strconv.Atoi(parts[1])
			}
			if addSlot > 0 && addCount > 0 {
				err := tickets.Repo.CreateSlots("", addSlot, addCount)
				if err != nil {
					data.ErrorMsg = err.Error()
				}
			}
		}

		if r.FormValue("password") == data.Password && data.ErrorMsg == "" {
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
				// blow out the cache (use the low-request admin handler as cheap cache invalidation)
				tickets.Repo.ClearCache()
			}
		} else {
			data.ErrorMsg = "Invalid password"
		}

	}

	log.Println("TicketAdminHandler", data.ErrorMsg)
	Render(w, "admin.html", data)
	return
}

// TicketAdminExpiresHandler expires tickets, send email notices
func TicketAdminExpiresHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain")
	if r.URL.Query().Get("pwd") != os.Getenv("ADVLIGHT_PASSWORD") {
		w.Write([]byte("invalid password"))
		return
	}
	guests, err := tickets.Repo.GetExpiredGuests("1 hour")
	if err != nil {
		panic(err)
	}
	w.Header().Add("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("notifying %d guests of expiration\n\n", len(guests))))
	for _, g := range guests {
		slot := g.Tickets[0]
		em := tickets.ExpirationEmail(*g, slot.Slot)
		subject := fmt.Sprintf("Your %s ticket request expired (%s)", config.EventName, slot.Slot.Format("Jan 02, 3:04pm"))
		err = tickets.Mailer.Send(g.Email, subject, em)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("ERROR %s %s", g.Email, err.Error())))
		}
		err = tickets.Repo.CancelTicket(g, slot.Slot)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("DB-ERROR %s %s", g.Email, err.Error())))
		}
	}

}

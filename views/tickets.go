package views

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/blit/advlight/tickets"
	"github.com/go-chi/chi"
)

func TicketShowHandler(w http.ResponseWriter, r *http.Request) {
	guestID := chi.URLParam(r, "guestID")
	ticketID := chi.URLParam(r, "ticketID")

	data := struct {
		ErrorMsg string
		Ticket   *tickets.Ticket
		Guest    *tickets.Guest
	}{
		"",  // ErrorMsg
		nil, // Ticket
		nil, // Guest
	}

	slot, err := strconv.ParseInt(ticketID, 10, 64)
	slotTime := time.Unix(slot, 0)
	if err != nil {
		log.Printf("TicketShowHandler.invalid_ticket %s %v", ticketID, err)
		data.ErrorMsg = fmt.Sprintf("%s is not a valid ticket", ticketID)
		Render(w, "ticket.html", data)
		return
	}

	guest, err := tickets.Repo.GetGuest(guestID)
	if err != nil {
		data.ErrorMsg = err.Error()
		Render(w, "ticket.html", data)
		return
	}
	data.Guest = guest
	for idx, t := range guest.Tickets {
		if t.Slot.Equal(slotTime) {
			data.Ticket = &(guest.Tickets[idx])
			break
		}
	}
	if data.Ticket == nil {
		data.ErrorMsg = "Sorry, this does not appear to be a valid ticket id, please check your link and try again"
	} else if !guest.Verified {
		tickets.Repo.VerifyGuest(guest)
	}

	Render(w, "ticket.html", data)
	return

}

func TicketIndexHandler(w http.ResponseWriter, r *http.Request) {
	guestID := chi.URLParam(r, "guestID")
	slots, err := tickets.Repo.GetSlots()
	if err != nil {
		RenderError(w, err)
		return
	}
	data := struct {
		Slots            []tickets.Slot
		SelectedSlot     int64
		CancelSlot       int64
		ErrorMsg         string
		SuccessMsg       string
		SentEmailConfirm bool
		Email            string
		Guest            *tickets.Guest
	}{
		slots, // Sots
		0,     // SelectSlot
		0,     // CancelSlot
		"",    // ErrorMsg
		"",    // SuccessMsg
		false, // SentEmailConfirm
		"",    // Email
		nil,   // Guest
	}
	// populate view data
	if guestID != "" {
		guest, err := tickets.Repo.GetGuest(guestID)
		if err != nil {
			data.ErrorMsg = err.Error()
			Render(w, "index.html", data)
			return
		}
		if !guest.Verified {
			tickets.Repo.VerifyGuest(guest)
		}
		data.Email = guest.Email
		data.Guest = guest
		if len(guest.Tickets) > 0 {
			data.SelectedSlot = guest.Tickets[0].Slot.Unix()
		}
	}

	if r.Method == "POST" {
		var err error
		data.Email = strings.TrimSpace(strings.ToLower(r.FormValue("email")))
		data.SelectedSlot, err = strconv.ParseInt(r.FormValue("slot"), 10, 64)
		if err != nil {
			data.ErrorMsg = err.Error()
			Render(w, "index.html", data)
			return
		}
		if r.FormValue("cancelslot") != "" {
			data.CancelSlot, err = strconv.ParseInt(r.FormValue("cancelslot"), 10, 64)
		}
		if err != nil {
			data.ErrorMsg = err.Error()
			Render(w, "index.html", data)
			return
		}
	}

	// cancel a ticket/slot -- guest musgt be set
	if r.Method == "POST" && data.CancelSlot > 0 {
		if data.Guest == nil {
			// guest must be set, but we are not going to leak that to the script kiddies
			Render(w, "index.html", data)
			return
		}
		slotTime := time.Unix(int64(data.CancelSlot), 0)
		err = tickets.Repo.CancelTicket(data.Guest, slotTime)
		log.Printf("TicketIndexHandler::CancelSlot %s %d %v %v", data.Guest.Email, data.CancelSlot, slotTime, err)
		// reload the guest
		data.Guest, _ = tickets.Repo.GetGuest(data.Guest.ID)
		if err != nil {
			data.ErrorMsg = err.Error()
		} else {
			data.SuccessMsg = "Ticket Cancelled"
		}
		Render(w, "index.html", data)
		return
	}

	// update or book a slot/ticket
	if r.Method == "POST" && data.SelectedSlot > 0 {
		slotTime := time.Unix(int64(data.SelectedSlot), 0)
		guest := &tickets.Guest{Email: data.Email}
		err = guest.Validate()
		log.Printf("TicketIndexHandler::SelectedSlot %s %d %v %v", data.Email, data.SelectedSlot, slotTime, err)
		if err != nil {
			data.ErrorMsg = err.Error()
			Render(w, "index.html", data)
			return
		}
		err = tickets.Repo.CreateGuest(guest)
		if err != nil {
			data.ErrorMsg = err.Error()
			Render(w, "index.html", data)
			return
		}
		err = tickets.Repo.AssignTicket(guest, slotTime)
		if err != nil {
			data.ErrorMsg = err.Error()
			Render(w, "index.html", data)
			return
		}
		// if we have a guest we need to reload it to relect new ticket times
		if guest.ID != "" {
			data.Guest, err = tickets.Repo.GetGuest(guest.ID)
		}

		em := tickets.ConfirmationEmail(*guest, slotTime)
		err = tickets.SendEmail(guest.Email, em)
		if err != nil {
			data.ErrorMsg = err.Error()
			Render(w, "index.html", data)
			return
		}
		data.SentEmailConfirm = true
		Render(w, "index.html", data)
		return
	}

	// render default (GET)
	Render(w, "index.html", data)
	return

}

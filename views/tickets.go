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
	// TODO remolve oopsSlotTime next year (I goofed and made all tickets for 2017 in 2018)
	oopsSlotTime := time.Unix(slot+(60*60*24*365), 0)
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
		if t.Slot.Equal(slotTime) || t.Slot.Equal(oopsSlotTime) {
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
	data := struct {
		Slots            []tickets.Slot
		SelectedSlot     int64
		CancelSlot       int64
		ErrorMsg         string
		SuccessMsg       string
		SentEmailConfirm bool
		Email            string
		EventCode        string
		Guest            *tickets.Guest
	}{
		nil,   // Slots
		0,     // SelectSlot
		0,     // CancelSlot
		"",    // ErrorMsg
		"",    // SuccessMsg
		false, // SentEmailConfirm
		"",    // Email
		r.URL.Query().Get("event"), // EventCode
		nil, // Guest
	}
	// populate view data
	if guestID != "" {
		guest, err := tickets.Repo.GetGuest(guestID)
		if err != nil {
			data.ErrorMsg = err.Error()
		} else {
			// set guest info
			if !guest.Verified {
				tickets.Repo.VerifyGuest(guest)
			}
			data.Email = guest.Email
			data.Guest = guest
			if len(guest.Tickets) > 0 {
				data.SelectedSlot = guest.Tickets[0].Slot.Unix()
			}
		}
	}

	defer func() {
		// remove slots from log, too noisy
		data.Slots = nil
		log.Printf("TicketIndexHandler %+v\n", data)
	}()

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
	// event codes affect the slots so need to be processed before calling GetSlots
	// get the eventcode & slots
	if strings.TrimSpace(data.EventCode) == "" {
		data.EventCode = r.FormValue("eventcode")
	}
	// see if the event code is being set to a new one
	if r.FormValue("seteventcode_new") != "" {
		data.EventCode = r.FormValue("seteventcode_new")
	}
	// see if the event code is being cleared
	if r.FormValue("seteventcode_new") == "!clr" {
		data.EventCode = ""
	}

	data.EventCode = strings.TrimSpace(strings.ToLower(data.EventCode))
	slots, err := tickets.Repo.GetSlots(data.EventCode)
	if err != nil {
		RenderError(w, err)
		return
	}

	// only show current slots
	cutOff := time.Now().Add(-(time.Minute * 30))
	for {
		if len(slots) < 1 {
			break
		}
		if slots[0].Slot.YearDay() > cutOff.YearDay() || (slots[0].Slot.YearDay() == cutOff.YearDay() && slots[0].Slot.Hour() >= cutOff.Hour()) {
			//log.Printf("breaking slot cutoff %+v; cutoff %+v  %d>%d", slots[0].Slot, cutOff, slots[0].Slot.Hour(), cutOff.Hour())
			break
		}
		//log.Printf("removing %+v; cutoff %+v", slots[0].Slot, cutOff)
		slots = slots[1:]
	}

	if len(slots) < 1 && data.EventCode != "" {
		data.ErrorMsg = fmt.Sprintf("%s is an invalid event code or is no longer valid", data.EventCode)
		data.EventCode = ""
		slots, err = tickets.Repo.GetSlots(data.EventCode)
		if err != nil {
			RenderError(w, err)
			return
		}
	}
	data.Slots = slots

	// if we are just setting the event, we can exit now
	if r.FormValue("seteventcode") != "" {
		Render(w, "index.html", data)
		return
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

		// captcha should be used for unvalidated guests
		captchResp := strings.TrimSpace(r.FormValue("g-recaptcha-response"))
		if (guestID == "" && !guest.Verified) || captchResp != "" {
			_, err := tickets.CAPTCHAVerify(captchResp, r.RemoteAddr)
			if err != nil && !tickets.CAPTCHADisabled {
				data.ErrorMsg = "CAPTCHAVerify error: " + err.Error()
				Render(w, "index.html", data)
				return
			}
		}

		err = tickets.Repo.AssignTicket(guest, slotTime, data.EventCode)
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
		err = tickets.Mailer.Send(guest.Email, "Confirm and View your Bayside Christmas Drive-Thru Tickets", em)
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

func TicketFacesHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Tickets  []tickets.Ticket
		ErrorMsg string
	}{
		nil, // Slots
		"",  // ErrorMsg
	}
	// populate view data
	days, err := tickets.Repo.GetSlotDates()
	if err == nil {
		data.Tickets = make([]tickets.Ticket, len(days))
		for i, d := range days {
			data.Tickets[i] = tickets.Ticket{Slot: d}
		}
	} else {
		data.ErrorMsg = "error loading data " + err.Error()
	}
	Render(w, "ticketfaces.html", data)
	return
}

package tickets

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

var Repo *repo
var HostName string
var DatabaseURL string

func init() {
	var err error
	DatabaseURL = os.Getenv("ADVLIGHT_DATABASE_URL")
	Repo = &repo{}
	Repo.db, err = sql.Open("postgres", DatabaseURL)
	if err != nil {
		log.Fatalln(err)
	}
	if strings.EqualFold(os.Getenv("ADVLIGHT_ENV"), "production") {
		HostName = "https://bcatickets.blit.com"
	} else {
		HostName = "http://localhost:8080"
	}
}

type Guest struct {
	ID        string
	Email     string
	Verified  bool
	IPAddress string

	Tickets []Ticket
}

func (g *Guest) Validate() error {
	if !strings.Contains(g.Email, "@") || !strings.Contains(g.Email, ".") {
		return fmt.Errorf("invalid email address")
	}
	return nil
}

func (g Guest) GetToken() string {
	return strings.Replace(g.ID, "-", "", -1)
}

func (g Guest) GetTicketURL(slot time.Time) string {
	return HostName + "/" + g.GetToken() + "/ticket/" + strconv.Itoa(int(slot.Unix()))
}

type Ticket struct {
	Slot    time.Time
	Number  int64
	GuestID string
}

type Slot struct {
	Slot             time.Time
	AvailableTickets int64
}

type repo struct {
	sync  sync.Mutex
	db    *sql.DB
	cache struct {
		slots []Slot
	}
}

func (r *repo) GetSlots() ([]Slot, error) {
	r.sync.Lock()
	defer r.sync.Unlock()
	if r.cache.slots != nil {
		log.Println("slots from cache", len(r.cache.slots))
		return r.cache.slots, nil
	}
	rows, err := r.db.Query(`select slot,count(*) from tickets where guest_id is null group by slot order by slot;`)
	if err != nil {
		return nil, err
	}
	slots := make([]Slot, 0)
	defer rows.Close()
	for rows.Next() {
		slot := &Slot{}
		rows.Scan(&(slot.Slot), &(slot.AvailableTickets))
		slots = append(slots, *slot)
	}
	r.cache.slots = slots
	return slots, nil
}

func (r *repo) GetGuest(guestID string) (*Guest, error) {
	log.Println(`GetGuest`, guestID)
	rows, err := r.db.Query(`select g.id,g.email,g.verified,t.slot,t.num from guests g left join tickets t on (g.id=t.guest_id) where g.id=$1 order by t.slot;`, guestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var g *Guest
	for rows.Next() {
		var (
			tslot pq.NullTime
			tnum  sql.NullInt64
		)

		if g == nil {
			g = &Guest{
				Tickets: make([]Ticket, 0),
			}
		}
		err = rows.Scan(&(g.ID), &(g.Email), &(g.Verified), &tslot, &tnum)
		if err != nil {
			return nil, err
		}
		if tslot.Valid {
			g.Tickets = append(g.Tickets, Ticket{
				Slot:    tslot.Time,
				Number:  tnum.Int64,
				GuestID: g.ID,
			})
		}
	}
	if g == nil {
		return nil, fmt.Errorf("Unable to locate your guest/ticket ID, please check your link and try again")
	}

	return g, nil
}

func (r *repo) VerifyGuest(g *Guest) error {
	_, err := r.db.Exec("update guests set verified=true where id=$1 and verified=false", g.ID)
	if err != nil {
		g.Verified = true
	}
	log.Printf("VerifyGuest %s %s, %v", g.ID, g.Email, err)
	return err
}

func (r *repo) CancelTicket(g *Guest, slot time.Time) error {
	log.Printf("CancelTicket %s %s, %v", g.ID, g.Email, slot)
	// cancel any tickets the guest would already have on this day
	_, err := r.db.Exec("update tickets set guest_id = null where guest_id=$1 and slot::date = $2::date", g.ID, slot)
	if err != nil {
		return err
	}
	r.sync.Lock()
	r.cache.slots = nil // bust the cache :(
	r.sync.Unlock()

	return nil
}

func (r *repo) AssignTicket(g *Guest, slot time.Time) error {
	log.Printf("AssignTicket %s %s, %v", g.ID, g.Email, slot)
	// check to see if guest already has a ticket for this day
	rows, err := r.db.Query(`select count(*) as tickets, count(*) filter (where slot=$2) as inslot from tickets where guest_id=$1 and slot::date=$2::date;`, g.ID, slot)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		// aggregate query should only return 1 row
		var numtix, slottix int
		rows.Scan(&numtix, &slottix)
		if slottix > 0 {
			return nil // guest already has ticket in slot
		}
		if numtix > 0 {
			// cancel tix for guest
			err := r.CancelTicket(g, slot)
			if err != nil {
				return err
			}
		}
	}
	rows.Close()
	rows, err = r.db.Query(`
	WITH avail AS (
		SELECT slot,num
		FROM   tickets
		WHERE  guest_id is null AND slot=$2
		LIMIT  1 FOR UPDATE          
		)
 	UPDATE tickets t
 	SET    guest_id = $1
 	FROM   avail
 	WHERE  t.slot = avail.slot and t.num = avail.num RETURNING t.num;`, g.ID, slot)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return fmt.Errorf("Sorry, just ran out of tickets.  Please try again in a few moments")
	}
	var tnum int64
	rows.Scan(&tnum)
	if r.cache.slots != nil {

		for idx, cslot := range r.cache.slots {

			if cslot.Slot.Equal(slot) {
				fmt.Printf("MATCH %v == %v\n", cslot, slot)
				r.sync.Lock()
				match := &(r.cache.slots[idx])
				match.AvailableTickets = match.AvailableTickets - 1
				r.sync.Unlock()
				break
			}
		}

	}
	return nil
}

func (r *repo) CreateGuest(g *Guest) error {
	g.Email = strings.TrimSpace(strings.ToLower(g.Email))
	var (
		rows *sql.Rows
		err  error
	)
	err = g.Validate()
	if err != nil {
		return err
	}

	rows, err = r.db.Query(`select id from guests where email=$1`, g.Email)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		rows.Scan(&(g.ID))
		return nil
	}
	rows.Close()
	rows, err = r.db.Query(`insert into guests(email) values($1) returning id;`, g.Email)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return fmt.Errorf("Unable to create new guest")
	}
	rows.Scan(&(g.ID))
	return nil
}

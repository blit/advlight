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
	Slot      time.Time
	Number    int64
	GuestID   string
	EventCode string
}

func (t Ticket) TicketImageURL() string {
	switch t.Slot.Weekday() {
	case time.Sunday:
		return "/assets/img/bgimg-0.png"
	case time.Monday:
		return "/assets/img/bgimg-1.gif"
	case time.Tuesday:
		return "/assets/img/bgimg-2.jpg"
	case time.Wednesday:
		return "/assets/img/bgimg-3.jpg"
	case time.Thursday:
		return "/assets/img/bgimg-4.gif"
	case time.Friday:
		return "/assets/img/bgimg-5.gif"
	case time.Saturday:
		return "/assets/img/bgimg-6.gif"
	}
	return "/assets/img/bgimg-0.png"
}

type Slot struct {
	Slot             time.Time
	AvailableTickets int64
}

type SlotStat struct {
	Slot             time.Time
	NumberTickets    int64
	AvailableTickets int64
	EventCode        string
}

type repo struct {
	sync  sync.Mutex
	db    *sql.DB
	cache struct {
		slots map[string][]Slot // key is eventcode
	}
}

func (r *repo) GetSlots(eventCode string) ([]Slot, error) {
	eventCode = strings.TrimSpace(strings.ToLower(eventCode))
	r.sync.Lock()
	defer r.sync.Unlock()
	if r.cache.slots != nil {
		slots, ok := r.cache.slots[eventCode]
		if ok {
			log.Println("slots from cache", eventCode, len(slots))
			return slots, nil
		}
	}
	rows, err := r.db.Query(`select coalesce(event_code,''),slot,count(*) from tickets where guest_id is null group by event_code,slot order by event_code,slot;`)
	if err != nil {
		return nil, err
	}
	r.cache.slots = make(map[string][]Slot)
	defer rows.Close()
	for rows.Next() {
		var ecode string
		slot := &Slot{}
		rows.Scan(&ecode, &(slot.Slot), &(slot.AvailableTickets))
		_, ok := r.cache.slots[ecode]
		if !ok {
			r.cache.slots[ecode] = make([]Slot, 0)
		}
		r.cache.slots[ecode] = append(r.cache.slots[ecode], *slot)
	}
	slots, ok := r.cache.slots[eventCode]
	if !ok {
		slots = make([]Slot, 0)
	}
	keys := make([]string, 0)
	for k, v := range r.cache.slots {
		keys = append(keys, fmt.Sprintf("%s:%d", k, len(v)))
	}
	log.Println("built cache", keys)
	return slots, nil
}

func (r *repo) GetGuest(guestID string) (*Guest, error) {
	log.Println(`GetGuest`, guestID)
	rows, err := r.db.Query(`select g.id,g.email,g.verified,t.slot,t.num,t.event_code from guests g left join tickets t on (g.id=t.guest_id) where g.id=$1 order by t.slot;`, guestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var g *Guest
	for rows.Next() {
		var (
			tslot  pq.NullTime
			tnum   sql.NullInt64
			tevent sql.NullString
		)

		if g == nil {
			g = &Guest{
				Tickets: make([]Ticket, 0),
			}
		}
		err = rows.Scan(&(g.ID), &(g.Email), &(g.Verified), &tslot, &tnum, &tevent)
		if err != nil {
			return nil, err
		}
		if tslot.Valid {
			g.Tickets = append(g.Tickets, Ticket{
				Slot:      tslot.Time,
				Number:    tnum.Int64,
				GuestID:   g.ID,
				EventCode: tevent.String,
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

func (r *repo) AssignTicket(g *Guest, slot time.Time, eventCode string) error {
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

	if eventCode == "" {
		rows, err = r.db.Query(`
			WITH avail AS (
				SELECT slot,num
				FROM   tickets
				WHERE  guest_id is null AND slot=$2 AND event_code IS NULL
				LIMIT  1 FOR UPDATE          
				)
			 UPDATE tickets t
			 SET    guest_id = $1
			 FROM   avail
			 WHERE  t.slot = avail.slot and t.num = avail.num RETURNING t.num;`, g.ID, slot)
	} else {
		rows, err = r.db.Query(`
			WITH avail AS (
				SELECT slot,num
				FROM   tickets
				WHERE  guest_id is null AND slot=$2 AND event_code = $3
				LIMIT  1 FOR UPDATE          
				)
			 UPDATE tickets t
			 SET    guest_id = $1
			 FROM   avail
			 WHERE  t.slot = avail.slot and t.num = avail.num RETURNING t.num;`, g.ID, slot, eventCode)
	}
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
		slots, ok := r.cache.slots[eventCode]
		if ok {
			for idx, cslot := range slots {

				if cslot.Slot.Equal(slot) {
					fmt.Printf("MATCH %v == %v\n", cslot, slot)
					r.sync.Lock()
					match := &(slots[idx])
					match.AvailableTickets = match.AvailableTickets - 1
					r.sync.Unlock()
					break
				}
			}
		}
	}
	return nil
}

func (r *repo) CreateGuest(g *Guest) error {
	log.Printf("CreateGuest %+v\n", g)
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

// GetSlotsStats gets all slots, not cached because it is behind an admin screen
func (r *repo) GetSlotsStats() ([]SlotStat, error) {
	log.Println("GetSlotsStats")
	rows, err := r.db.Query(`select coalesce(event_code,''),slot,count(*), count(*) filter(where guest_id is null) from tickets group by event_code,slot order by event_code,slot;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	slots := make([]SlotStat, 0)
	for rows.Next() {
		slot := &SlotStat{}
		rows.Scan(&(slot.EventCode), &(slot.Slot), &(slot.NumberTickets), &(slot.AvailableTickets))
		slots = append(slots, *slot)
	}
	return slots, nil
}

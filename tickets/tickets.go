package tickets

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blit/advlight/config"

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
	HostName = strings.TrimSpace(config.HostName)
	if HostName == "" {
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

func (g Guest) GetGuestURL() string {
	return HostName + "/" + g.GetToken()
}

type Ticket struct {
	Slot      time.Time
	Number    int64
	GuestID   string
	EventCode string
}

func (t Ticket) TicketImageURL() string {
	daynum := t.Slot.Day()
	if daynum == 9 {
		daynum = 15 // show the kitty
	} else if daynum > 16 {
		daynum = daynum - 16
	}
	return fmt.Sprintf("/assets/img/bgimg-%d.jpg", daynum)
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

func (r *repo) CreateSlots(eventCode string, ts, count int) error {
	log.Println(`CreateSlots`, eventCode, ts, count)
	if count > 100 { // safety
		return fmt.Errorf("%d is too many", count)
	}
	eventCode = strings.TrimSpace(strings.ToLower(eventCode))
	_, err := r.db.Exec(`
		with slot as (
		  select TIMESTAMP WITH TIME ZONE 'epoch' + $1 * INTERVAL '1 second' as slot
		), max_ticket_num as (
			select max(num)::integer as num from slot,tickets t where t.slot=slot.slot
		), ticket_numbers as (
			select num.num from max_ticket_num,generate_series(max_ticket_num.num+1, max_ticket_num.num+$2) num
		) insert into tickets(event_code,slot, num) (select NULLIF($3,''), slot.slot, ticket_numbers.num from slot cross join ticket_numbers);
	`, ts, count, eventCode)
	if err != nil {
		return err
	}
	// changed the db, so lets blow out the cache
	r.sync.Lock()
	r.cache.slots = nil
	r.sync.Unlock()
	return nil
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

func (r *repo) GetExpiredGuests(age string) ([]*Guest, error) {
	log.Println(`GetExpiredGuests`, age)
	rows, err := r.db.Query(`select g.id,g.email,g.verified,t.slot,t.num,t.event_code from guests g join tickets t on (g.id=t.guest_id) where g.verified = false and g.created_at<(current_timestamp-$1::interval) order by g.id,t.slot;`, age)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	guests := make([]*Guest, 0)
	for rows.Next() {
		var (
			tslot  pq.NullTime
			tnum   sql.NullInt64
			tevent sql.NullString
		)

		g := &Guest{
			Tickets: make([]Ticket, 0),
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
		if len(guests) > 0 && guests[len(guests)-1].ID == g.ID {
			if len(g.Tickets) > 0 {
				existing := guests[len(guests)-1]
				existing.Tickets = append(existing.Tickets, g.Tickets[0])
			}
		} else {
			guests = append(guests, g)
		}
	}
	return guests, nil
}

func (r *repo) GetEventCodeGuests(eventCode string) ([]*Guest, error) {
	log.Println(`GetEventCodeGuests`, eventCode)
	rows, err := r.db.Query(`select g.id,g.email,g.verified from guests g join tickets t on (g.id=t.guest_id) where t.event_code=$1;`, eventCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	guests := make([]*Guest, 0)
	for rows.Next() {
		g := &Guest{
			Tickets: make([]Ticket, 0),
		}
		err = rows.Scan(&(g.ID), &(g.Email), &(g.Verified))
		if err != nil {
			return nil, err
		}
		guests = append(guests, g)
	}
	return guests, nil
}

func (r *repo) GetGuestsByDate(date string) ([]*Guest, error) {
	log.Println(`GetGuestsByDate`, date)
	rows, err := r.db.Query(`select g.id,g.email,g.verified from guests g join tickets t on (g.id=t.guest_id) where t.slot::date=$1::date;`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	guests := make([]*Guest, 0)
	for rows.Next() {
		g := &Guest{
			Tickets: make([]Ticket, 0),
		}
		err = rows.Scan(&(g.ID), &(g.Email), &(g.Verified))
		if err != nil {
			return nil, err
		}
		guests = append(guests, g)
	}
	return guests, nil
}

//select count(*) from guests g join tickets t on g.id=t.guest_id where verified = false and created_at<(current_timestamp-'1 hour'::interval);

//select array_agg(g.email) from guests g join tickets t on g.id=t.guest_id and t.slot::time = '21:30'::time;

//select count(*) from guests g join tickets t on g.id=t.guest_id and g.verified is null

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
	// cancel any tickets the guest would already have on this
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
					if match.AvailableTickets < 1 {
						// slot needs to be removed, so we'll just blow out the cache
						r.cache.slots = nil
					}
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
	rows, err := r.db.Query(`select coalesce(event_code,''),slot,count(*), count(*) filter(where guest_id is null) from tickets where slot>=((now() AT TIME ZONE 'PST')-'30 minutes'::interval) group by event_code,slot order by slot,event_code NULLS LAST;`)
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

var getSlotDatesCache []time.Time

// GetSlotDates returns a cached array of time obj, 1 for each day there is a slot
func (r *repo) GetSlotDates() ([]time.Time, error) {
	if getSlotDatesCache != nil {
		return getSlotDatesCache, nil
	}
	// race cond could exist, not big deal right now
	log.Println("GetSlotsStats")
	rows, err := r.db.Query(`select slot::date from tickets group by 1 order by 1;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	getSlotDatesCache = make([]time.Time, 0)
	for rows.Next() {
		var dtstr string
		rows.Scan(&dtstr)
		dt, err := time.Parse(time.RFC3339, dtstr)
		if err != nil {
			return getSlotDatesCache, err
		}
		getSlotDatesCache = append(getSlotDatesCache, dt)
	}
	return getSlotDatesCache, nil
}

// ToCSV writes the database to csv
func (r *repo) ToCSV(w io.Writer) error {
	log.Println("Repo ToCSV")
	rows, err := r.db.Query(`select g.email,g.created_at,t.updated_at,g.verified,coalesce(g.ip_address,'0.0.0.0'),t.slot,coalesce(t.event_code,'') from guests g join tickets t on g.id=t.guest_id;`)
	if err != nil {
		return err
	}
	rec := []string{"email", "created", "updated", "verified", "ip_address", "slot", "event_code"}
	wc := csv.NewWriter(w)
	wc.Write(rec) // write the headers, rec will be reused for rows

	defer rows.Close()
	defer wc.Flush()
	for rows.Next() {
		for i := 0; i < 7; i++ {
			err := rows.Scan(&rec[0], &rec[1], &rec[2], &rec[3], &rec[4], &rec[5], &rec[6])
			if err != nil {
				err = fmt.Errorf("error scanning col %d: %s", i, err.Error())
				log.Println(err)
				wc.Write([]string{err.Error()})
				return err
			}
		}
		wc.Write(rec)
	}
	return nil
}

func (r *repo) ClearCache() {
	r.sync.Lock()
	r.cache.slots = nil
	r.sync.Unlock()
}

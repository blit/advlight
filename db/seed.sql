create extension if not exists citext;
create extension if not exists pgcrypto;

create table guests (
  id uuid PRIMARY key default gen_random_uuid(),
  created_at timestamptz not null default current_timestamp,
  email citext not null,
  verified bool not null default false,
  ip_address inet
);
create unique index guests_email_key on guests(email);

-- timestampslot ensures that a timeslot is in the correct date/time contraints
-- must start at top of hour or half hour
-- and dates are from 2017-11-26-2017-12-31 with none on the 9th or 24th
CREATE DOMAIN timestampslot AS timestamptz
CHECK(
  (to_char(VALUE,'MIUS') = '00000000' OR to_char(VALUE,'MIUS') = '30000000')
  and (VALUE::date!='2017-12-09' and VALUE::date!='2017-12-24')
  and (VALUE::date>='2017-11-26' and VALUE::date<='2017-12-31')
);

create table tickets (
  slot timestampslot not null,
  num integer not null,
  updated_at timestamptz not null default current_timestamp,
  guest_id uuid references guests(id) on delete set null on update cascade,
  PRIMARY KEY (slot,num)
);
create index tickets_guest_id_fkey on tickets(guest_id);

with days as (
select day from generate_series(
  '2017-11-26 18:30:00'::timestamptz,
  '2017-12-31 21:30:00'::timestamptz,
  '30 minutes'::interval
) ms(day) where (day::time>='18:30'::time and day::time<='21:30' and day::date!='2017-12-09' and day::date!='2017-12-24')
), ticket_numbers as (
  select num from generate_series(1,150) num
) insert into tickets(slot, num) (select days.day, ticket_numbers.num from days cross join ticket_numbers);

alter table tickets add column event_code citext;
create index tickets_event_code_key on tickets(event_code);
update tickets set event_code = 'staff' where slot::date='2017-11-26';
update tickets set event_code = 'mcc' where slot='2017-12-03 18:30:00' and num in(select subq.num from tickets subq where subq.slot='2017-12-03 18:30:00' and subq.guest_id is null order by num limit 100);




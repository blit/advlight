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

-- timestampslot ensures that a timeslot time is either top of hour or half hour
CREATE DOMAIN timestampslot AS timestamptz
CHECK(
  (to_char(VALUE,'MIUS') = '00000000' OR to_char(VALUE,'MIUS') = '30000000')
);

create table tickets (
  slot timestampslot not null,
  num integer not null,
  updated_at timestamptz not null default current_timestamp,
  guest_id uuid references guests(id) on delete set null on update cascade,
  event_code citext,
  PRIMARY KEY (slot,num)
);
create index tickets_guest_id_fkey on tickets(guest_id);

with days as (
select day from generate_series(
  '2019-12-01 18:00:00'::timestamptz,
  '2019-12-31 21:00:00'::timestamptz,
  '30 minutes'::interval
) ms(day) where (day::time>='18:00'::time and day::time<='21:00' and day::date!='2017-12-05' and day::date!='2017-12-24')
), ticket_numbers as (
  select num from generate_series(1,180) num
) insert into tickets(slot, num) (select days.day, ticket_numbers.num from days cross join ticket_numbers);
delete from tickets where (slot::time<'18:30') and slot::date in('2019-12-07','2019-12-14','2019-12-21','2019-12-28');
update tickets set event_code = 'staff' where slot::date='2019-12-01';


-- chcclights
with days as (
select day from generate_series(
  '2019-11-24 18:00:00'::timestamptz,
  '2019-12-22  21:30:00'::timestamptz,
  '30 minutes'::interval
) ms(day) where (day::time>='18:00'::time and day::time<='21:30' and day::date not in ('2019-11-27','2019-11-30','2019-12-04','2019-12-07','2019-12-12','2019-12-13','2019-12-19','2019-12-20','2019-12-23','2019-12-24'))
), ticket_numbers as (
  select num from generate_series(1,250) num
) insert into tickets(slot, num) (select days.day, ticket_numbers.num from days cross join ticket_numbers);

delete from tickets where (slot::time<'18:30' or slot::time>'20:00') and (slot::date < '2019-12-06' or slot::date in('2019-12-08','2019-12-09','2019-12-10'));
delete from tickets where (slot::time<'19:00' or slot::time>'20:30') and slot::date in('2019-12-11','2019-12-18');
delete from tickets where (slot::time<'20:00' or slot::time>'21:30') and slot::date in('2019-12-14','2019-12-21');
delete from tickets where (slot::time<'18:30' or slot::time>'21:00') and slot::date in('2019-12-15');
delete from tickets where (slot::time<'18:00' or slot::time>'21:00') and slot::date in('2019-12-16','2019-12-17','2019-12-22');

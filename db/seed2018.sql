truncate table tickets;

with days as (
select day from generate_series(
  '2017-11-25 18:00:00'::timestamptz,
  '2017-12-31 21:00:00'::timestamptz,
  '30 minutes'::interval
) ms(day) where (day::time>='18:00'::time and day::time<='21:00' and day::date NOT IN('2017-12-01', '2017-12-06', '2017-12-07', '2017-12-15', '2017-12-18', '2017-12-19', '2017-12-21', '2017-12-23', '2017-12-24'))
), ticket_numbers as (
  select num from generate_series(1,150) num
) insert into tickets(slot, num) (select days.day, ticket_numbers.num from days cross join ticket_numbers);

update tickets set event_code = 'staff' where slot::date='2017-11-25';
-- remove tickets for special events
delete from tickets where slot = '2017-12-02 18:30:00' and num>50;
delete from tickets where slot = '2017-12-11 18:30:00' and num>50;
delete from tickets where slot = '2017-12-11 19:00:00' and num>50;

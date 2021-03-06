# Simple Ticket Server for Bayside Christmas Lights

advlight is a Go application, to run in development you need:

1. Go installed (latest version perferred)
1. Postgresql installed (latest version perferred)
1. Create the DB and run seed.sql 
1. Point `ADVLIGHT_DATABASE_URL` env to DB

Production:
```
# package assets & compile
go-bindata -o views/assets/bindata.go -pkg assets wwwroot/...
GOOS=linux go build advlight.go

# scp to prod with foolling env setup
ADVLIGHT_DATABASE_URL=[db_url]
ADVLIGHT_ENV=production
ADVLIGHT_SMTP=[username,password,host,port]
ADVLIGHT_GAID=[captcha] # run with -nocaptcha flag to bypass captcha in dev
ADVLIGHT_RECAPTCHA_SECRET=[captcha]

# current deploy procedure
scp advlight bcatickets.blit.com:advlight_update
ssh bcatickets.blit.com '~/advlight_deploy'

# copy production db
ssh -C bcatickets.blit.com "/usr/local/pgsql/bin/pg_dump -C -hlocalhost -Upostgres --no-owner --no-privileges advlight" | psql advlight
# dumb guests as csv
ssh bcatickets.blit.com "psql -hlocalhost -Upostgres advlight -c \"COPY (select * from guests) TO STDOUT WITH CSV\""

```

## LICENSE

All the files in this distribution are copyright (c) 2017 Blit, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.


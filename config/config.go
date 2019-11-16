package config

import "os"

var HostName = os.Getenv("ADVLIGHT_HOSTNAME")
var Port = os.Getenv("ADVLIGHT_PORT")
var ChurchName = os.Getenv("ADVLIGHT_CHURCHNAME")
var EventName = os.Getenv("ADVLIGHT_EVENTNAME")
var EventLink = os.Getenv("ADVLIGHT_EVENTLINK")
var EventBanner = os.Getenv("ADVLIGHT_EVENTBANNER")
var EventLogo = os.Getenv("ADVLIGHT_EVENTLOGO")
var EventAddress = os.Getenv("ADVLIGHT_EVENTADDRESS")
var DonateLink = os.Getenv("ADVLIGHT_DONATELINK")
var FavICO = os.Getenv("ADVLIGHT_FAVICON")

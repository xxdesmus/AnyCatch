package main

import (
	"appengine"
	"appengine/urlfetch"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/benjojo/maxminddb-golang"
	"github.com/codegangsta/martini"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

func init() {
	m := martini.Classic()
	m.Get("/discover/:ip", SendOutPings)

	m.Use(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Cache-Control", "public")
		res.Header().Add("X-Powered-By", "uW0t-m8")
	})

	http.Handle("/", m)
}

type Results struct {
	Geoip struct {
		Lati float64 `json:"lati"`
		Long float64 `json:"long"`
	} `json:"geoip"`
	Hit struct {
		Geoip struct {
			Lati float64 `json:"lati"`
			Long float64 `json:"long"`
		} `json:"geoip"`
		Name string `json:"name"`
	} `json:"hit"`
	Ip string `json:"ip"`
}

type GeoIPCity struct {
	Country struct {
		GeoNameID uint              `maxminddb:"geoname_id"`
		IsoCode   string            `maxminddb:"iso_code"`
		Names     map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		MetroCode uint    `maxminddb:"metro_code"`
		TimeZone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
}

type Worker struct {
	Name      string
	URL       string
	Latitude  float64
	Longitude float64
}

func SendOutPings(rw http.ResponseWriter, req *http.Request, params martini.Params) string {

	c := appengine.NewContext(req)

	SendBack := Results{}

	db, err := maxminddb.OpenGzip("GeoLite2-City.mmdb.gz")
	if err != nil {
		http.Error(rw, fmt.Sprintf("Error reading geoip db: %s", err), http.StatusInternalServerError)
	}
	defer db.Close()
	ip := net.ParseIP(params["ip"]).To4()

	if ip == nil {
		http.Error(rw, fmt.Sprintf("Not a valid IP4 %s", params["ip"]), http.StatusBadRequest)
		return ""
	}
	GIP := GeoIPCity{}
	db.Lookup(ip, &GIP)

	SendBack.Geoip.Lati = GIP.Location.Latitude
	SendBack.Geoip.Long = GIP.Location.Longitude

	workers := []Worker{
		{Name: "storm", URL: "storm.benjojo.co.uk:2374", Latitude: 33.9425, Longitude: -118.4080},
		{Name: "belle", URL: "belle.benjojo.co.uk:2374", Latitude: 40.6397, Longitude: -73.7788},
		{Name: "flora", URL: "flora.benjojo.co.uk:2374", Latitude: 49.6233, Longitude: 6.2044},
	}
	token := RandString(8)

	for _, v := range workers {

		client := urlfetch.Client(c)
		re, err := client.Get(fmt.Sprintf("http://%s/Send/%s/%s", v.URL, params["ip"], token))
		if err != nil {
			http.Error(rw, fmt.Sprintf("Cannot contact worker %s", err), http.StatusInternalServerError)
			return ""
		}
		if re.StatusCode != 200 {
			continue
		}
	}

	time.Sleep(time.Second * 3)

	for _, v := range workers {
		client := urlfetch.Client(c)
		re, err := client.Get(fmt.Sprintf("http://%s/Get", v.URL))
		if err != nil {
			continue
		}
		bytes, err := ioutil.ReadAll(re.Body)
		if err != nil {
			continue
		}
		tokens := make([]string, 0)
		json.Unmarshal(bytes, &tokens)
		if Contains(tokens, token) {
			SendBack.Hit.Name = v.Name
			SendBack.Hit.Geoip.Lati = v.Latitude
			SendBack.Hit.Geoip.Long = v.Longitude
			break
		}
	}

	output, _ := json.Marshal(SendBack)
	return string(output)
}

func RandString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}

func Contains(in []string, test string) bool {
	for _, v := range in {
		if v == test {
			return true
		}
	}
	return false
}
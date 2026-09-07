package lib

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mileusna/useragent"
)

func ExtractDevice(r *http.Request) string {
	uaString := r.UserAgent()
	if strings.TrimSpace(uaString) == "" {
		return "an unidentified machine"
	}

	ua := useragent.Parse(uaString)
	if ua.Name == "" && ua.OS == "" {
		return "an unidentified machine"
	}

	if ua.Name != "" && ua.OS != "" {
		return fmt.Sprintf("%s on %s", ua.Name, ua.OS)
	}
	if ua.OS != "" {
		return ua.OS
	}
	return ua.Name
}

func ExtractLocation(r *http.Request) string {
	city := r.Header.Get("CF-IPCity")
	if city == "" {
		city = r.Header.Get("X-AppEngine-City")
	}

	country := r.Header.Get("CF-IPCountry")
	if country == "" {
		country = r.Header.Get("X-AppEngine-Country")
	}

	city = strings.TrimSpace(city)
	country = strings.TrimSpace(country)

	if city != "" && country != "" {
		return fmt.Sprintf("%s, %s", city, country)
	}
	if country != "" {
		return country
	}
	if city != "" {
		return city
	}

	return "an undisclosed location"
}

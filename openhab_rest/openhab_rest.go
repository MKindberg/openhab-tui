package openhab_rest

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

type Mapping struct {
	Command string
	Label   string
}
type Item struct {
	Link  string
	State string
}
type Widget struct {
	Type       string
	Visibility bool
	Label      string
	Icon       string
	Mappings   []Mapping
	Item       Item
	Depth      int
	Actions    map[string]func(*Widget)
	Render     func(Widget) string
	Widgets    []Widget
}
type Page struct {
	Title   string
	Widgets []Widget
}
type Sitemap struct {
	Name     string
	Label    string
	Link     string
	Homepage Page
}

// Get sitemap from given ip with given name.
func Get_sitemap(ip string, name string) Sitemap {

	var body []byte
	var err error
	resp, err := http.Get("http://" + ip + ":8080/rest/sitemaps/" + name)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	if len(body) == 0 {
		log.Fatal("No sitemap found with the name " + name)
	}
	var data Sitemap
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Fatal(err)
	}

	return data

}

func Set_item(link string, state string) {
	resp, err := http.Post(link, "text/plain", bytes.NewBuffer([]byte(state)))
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
}

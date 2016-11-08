// +build !appengine

package main

import (
	"database/sql"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"encoding/json"

	"fmt"

	"net/url"

	"strings"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func init() {

	prepDB()

	router := mux.NewRouter()
	router.HandleFunc(`/recordlead`, JSONCatcher)
	router.HandleFunc(`/`, index)
	router.HandleFunc(`/{username}`, archive)
}

func main() {

	s := &http.Server{
		Addr:           ":3232",
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Fatal(s.ListenAndServe())
}

func index(w http.ResponseWriter, r *http.Request) {
	//login and choice functions go here
}

func archive(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	since, err := strconv.Atoi(query["since"][0])
	var leads []LeadDetails
	if since != 0 {
		leads = retrieveLeads(int64(since)) // Get all new leads since chosen time
	} else {
		leads = retrieveLeads(time.Now().UnixNano() - 24*time.Hour.Nanoseconds()) // Get all new leads from the last day
	}
	JSONLeads, err := json.Marshal(leads)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprint(w, string(JSONLeads))
}

// JSONCatcher stands ready to receive data from the Tampermonkey script
func JSONCatcher(w http.ResponseWriter, r *http.Request) {
	fmt.Println(`Caught new connection`)
	dc := json.NewDecoder(r.Body)
	leadRequest := *new(LeadRequest)

	if err := dc.Decode(&leadRequest); err != nil {
		log.Print(err.Error())
	}
	if leadRequest.UserName == `HenryRackley` {
		parseLeadDetails(leadRequest.Lead)
	}
}

func reduceURL(uri string) string {
	liURL, err := url.Parse(uri)
	if err != nil {
		log.Println(err.Error())
	}
	return fmt.Sprintf("%s://%s%s", liURL.Scheme, liURL.Host, liURL.Path)
}

func findCompany(leadTitle string, leadCompany string) string {
	if len([]rune(leadCompany)) > 3 {
		return leadCompany
	}
	if strings.Contains(leadTitle, ` at `) {
		return strings.Split(leadTitle, ` at `)[1]
	}
	return `Company Withheld`
}

func parseLeadDetails(l LeadDetails) {

	l.URL = reduceURL(l.URL)
	l.Company = findCompany(l.Title, l.Company)
	re := regexp.MustCompile("[^0-9]")
	l.Phone = re.ReplaceAllString(l.Phone, "")

	if len([]rune(l.Email)) < 3 {
		l.Email = `Email Withheld`
	}

	if len([]rune(l.Phone)) < 3 {
		l.Phone = `Phone Withheld`
	}

	db, err := sql.Open("sqlite3", "./leads.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("insert into leads(firstName, lastName, title, company, email, phone, url, created_at) values(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(l.FirstName, l.LastName, l.Title, l.Company, l.Email, l.Phone, l.URL, time.Now().UnixNano())
	if err != nil {
		log.Println(err.Error())
	} else {
		log.Printf("Inserted row for: %s\n", l.FirstName)
	}

	tx.Commit()
}

func retrieveLeads(since int64) []LeadDetails {
	fmt.Printf("Printing leads from %d\n", since)

	var leads []LeadDetails

	db, err := sql.Open("sqlite3", "./leads.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("select firstName, lastName, title, company, email, phone, url from leads where leads.created_at > ?", since)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var lead LeadDetails
		err = rows.Scan(&lead.FirstName, &lead.LastName, &lead.Title, &lead.Company, &lead.Email, &lead.Phone, &lead.URL)
		if err != nil {
			log.Println(err.Error())
		}
		leads = append(leads, lead)
	}
	return leads
}

func prepDB() {
	db, err := sql.Open("sqlite3", "./leads.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	qry := `create table if not exists leads (id integer not null primary key, firstName text, lastName text, title text, company text, email text, phone text, url text unique, created_at integer);`
	_, err = db.Exec(qry)
	if err != nil {
		log.Printf("%q: %s\n", err, qry)
		return
	}
}

type LeadRequest struct {
	UserName string      `json:"userName"`
	UserPass string      `json:"userPass"` // This currently doesn't do anything; authentication forthcoming.
	Lead     LeadDetails `json:"leadDetails"`
}

type LeadDetails struct {
	ID        int    `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Title     string `json:"title"`
	Company   string `json:"company"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	URL       string `json:"url"`
}

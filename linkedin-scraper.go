// Copyright (C) 2016 James Duerden (Xymist)

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Xymist/emailVerifier"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	fmt.Println("Booting...")

	prepDB()

	router := mux.NewRouter()
	router.HandleFunc(`/recordlead`, JSONCatcher)
	router.HandleFunc(`/archive`, archive)
	router.HandleFunc("/{rest:.*}", assets)
	router.HandleFunc(`/`, index)
	http.Handle("/", router)
}

func index(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/index.html")
}

func assets(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/"+r.URL.Path)
}

func archive(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	var since int
	if query["since"] != nil {
		since, err = strconv.Atoi(query["since"][0])
	} else {
		since = -1
	}
	var leads []LeadDetails
	if since >= 0 {
		leads = retrieveLeads(int64(since)) // Get all new leads since chosen time
	} else {
		leads = retrieveLeads(time.Now().UnixNano() - 24*time.Hour.Nanoseconds()) // Get all new leads from the last day
	}
	JSONLeads, err := json.Marshal(leads)
	if err != nil {
		log.Fatal(err)
	}
	if query["csv"] != nil {
		fmt.Printf("New CSV request beginning %s\n", query["since"][0])
		c := csv.NewWriter(w)
		var header []string
		header = append(header, "First Name", "Last Name", "Title", "Company", "Email", "Phone", "URL")
		c.Write(header)
		for _, l := range leads {
			var record []string
			record = append(record, l.FirstName)
			record = append(record, l.LastName)
			record = append(record, l.Title)
			record = append(record, l.Company)
			record = append(record, l.Email)
			record = append(record, l.Phone)
			record = append(record, l.URL)
			c.Write(record)
		}
		c.Flush()
	} else {
		fmt.Printf("New JSON request beginning %s\n", query["since"][0])
		w.Write([]byte(string(JSONLeads)))
	}
}

// JSONCatcher stands ready to receive data from the Tampermonkey script
func JSONCatcher(w http.ResponseWriter, r *http.Request) {
	dc := json.NewDecoder(r.Body)
	leadRequest := *new(LeadRequest)

	if err := dc.Decode(&leadRequest); err != nil {
		log.Print("Could not decode JSON: " + err.Error())
	}
	if leadRequest.UserName == `HenryRackley` {
		parseLeadDetails(leadRequest.Lead)
	}
}

func reduceURL(uri string) string {
	liURL, err := url.Parse(uri)
	if err != nil {
		log.Println("Could not parse URL: " + err.Error())
	}
	return fmt.Sprintf("%s://%s%s", liURL.Scheme, liURL.Host, liURL.Path)
}

func findCompany(leadTitle string, leadCompany string) string {
	if len([]rune(leadCompany)) > 3 {
		ltd, err := regexp.Compile(`\s(l|L)(t|T)(d|D).?`)
		if err != nil {
			return leadCompany
		}
		if ltd.MatchString(leadCompany) {
			fmt.Println("Stripped LTD")
			return ltd.ReplaceAllString(leadCompany, "")
		}
		return leadCompany
	}
	if strings.Contains(leadTitle, ` at `) {
		return strings.Split(leadTitle, ` at `)[1]
	}
	return ""
}

func stripTitle(leadTitle string) string {
	if strings.Contains(leadTitle, ` at `) {
		return strings.Split(leadTitle, ` at `)[0]
	}
	return leadTitle
}

func stripAndDissect(name string) (string, string) {
	honours := []string{
		"MA", "BA", "MSc", "BSc", "MEng",
		"FInstSMM", "MCIPS", "CEng", "MIMechE",
		"PhD", "hons"}
	clean, err := regexp.Compile(`[^A-Za-z0-9\s]`)
	if err != nil {
		fmt.Println("Regex failed")
	}
	splitName := strings.Split(name, " ")
	names := []string{}
	for _, w := range splitName {
		w = clean.ReplaceAllString(w, "")
		if stringSliceContains(honours, w) == false {
			names = append(names, w)
		}
	}
	return names[0], names[len(names)-1]
}

func stringSliceContains(slice []string, check string) bool {
	for _, elem := range slice {
		if elem == check {
			return true
		}
	}
	return false
}

func parseLeadDetails(l LeadDetails) {
	if l.FullName == "" {
		log.Println("Blank input received, skipping")
		return
	}

	l.FirstName, l.LastName = stripAndDissect(l.FullName)

	l.URL = reduceURL(l.URL)
	l.Company = findCompany(l.Title, l.Company)
	re := regexp.MustCompile("[^0-9]")
	l.Phone = re.ReplaceAllString(l.Phone, "")
	l.Title = stripTitle(l.Title)

	if len([]rune(l.Email)) < 3 {
		ea, err := emailVerifier.FindEmail(l.FirstName, l.LastName, l.Company)
		if err != nil {
			l.Email = ""
			log.Println(err.Error())
		} else {
			l.Email = ea
		}
	} else {
		if err := emailVerifier.VerifyEmail(l.Email); err != nil {
			l.Email = ""
			log.Println("Email invalid: " + err.Error())
		}
	}

	if len([]rune(l.Phone)) < 3 {
		l.Phone = ""
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

	stmt, err := tx.Prepare("insert into leads(firstName, lastName, title, company, email, phone, url, created_at, updated_at) values(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(l.FirstName, l.LastName, l.Title, l.Company, l.Email, l.Phone, l.URL, time.Now().UnixNano(), time.Now().UnixNano())
	tx.Commit()
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: leads.url") {
			updateLeadDetails(l, db)
		} else {
			log.Println("Problem inserting into database: " + err.Error())
		}
	} else {
		log.Printf("Inserted row for: %s %s\n", l.FirstName, l.LastName)
	}
}

func updateLeadDetails(l LeadDetails, db *sql.DB) {
	var oldData LeadDetails

	lead := db.QueryRow("select ID, firstName, lastName, company, email, phone, title, url from leads where leads.url = ? limit 1", l.URL)
	err := lead.Scan(&oldData.ID, &oldData.FirstName, &oldData.LastName, &oldData.Company, &oldData.Email, &oldData.Phone, &oldData.Title, &oldData.URL)
	if err != nil {
		log.Fatal(err)
	}

	if (l.FirstName != oldData.FirstName) || (l.LastName != oldData.LastName) || (l.Company != oldData.Company) || (l.Email != oldData.Email) || (l.Phone != oldData.Phone) || (l.Title != oldData.Title) {
		var (
			newCompany string
			newEmail   string
			newPhone   string
		)

		if l.Company != "" {
			newCompany = l.Company
		} else {
			newCompany = oldData.Company
		}

		if l.Email != "" {
			newEmail = l.Email
		} else {
			newEmail = oldData.Email
		}

		if l.Phone != "" {
			newPhone = l.Phone
		} else {
			newPhone = oldData.Phone
		}

		res, err := db.Exec("update leads set firstName = ?, lastName = ?, title = ?, company = ?, email = ?, phone = ?, updated_at = ? where url = ?", l.FirstName, l.LastName, l.Title, newCompany, newEmail, newPhone, time.Now().UnixNano(), oldData.URL)
		rowCnt, err := res.RowsAffected()
		if err != nil {
			log.Fatal(err)
		}
		if err != nil || rowCnt == 0 {
			log.Printf("Could not update %s %s: %q", l.FirstName, l.LastName, err)
		} else {
			log.Printf("Updated details for %s %s.", l.FirstName, l.LastName)
		}
	}
}

func retrieveLeads(since int64) []LeadDetails {
	var leads []LeadDetails

	db, err := sql.Open("sqlite3", "./leads.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("select firstName, lastName, title, company, email, phone, url from leads where leads.updated_at > ?", since)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var lead LeadDetails
		err = rows.Scan(&lead.FirstName, &lead.LastName, &lead.Title, &lead.Company, &lead.Email, &lead.Phone, &lead.URL)
		if err != nil {
			log.Println("Could not scan existing leads: " + err.Error())
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

	qry := `create table if not exists leads (id integer not null primary key, firstName text, lastName text, title text, company text, email text, phone text, url text unique, created_at integer, updated_at integer);`
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
	FullName  string `json:"fullName"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Title     string `json:"title"`
	Company   string `json:"company"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	URL       string `json:"url"`
}

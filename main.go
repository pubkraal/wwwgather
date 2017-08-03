package main

import (
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var databaseClient *sql.DB

// stores all data sent to the server based on the identifier.  Adds a
// timestamp itself for some kind of ordering.
func store(w http.ResponseWriter, identifier string, data string) {
	q := "insert into items (ident, ts, data) VALUES ($1, $2, $3)"

	databaseClient.Query(q, identifier, time.Now(), data)

	w.Write([]byte("Thanks!"))

}

type listRendering struct {
	Ident string
	Rows  []string
}

// lists all stored items under the identifier
func listItems(w http.ResponseWriter, identifier string) {
	rows, err := databaseClient.Query("select data from items where ident = $1", identifier)
	if err != nil {
		w.WriteHeader(500)
		log.Println(err)
		return
	}
	defer rows.Close()

	var values []string

	for rows.Next() {
		var data string
		err := rows.Scan(&data)
		if err != nil {
			continue
		}

		values = append(values, data)
	}

	p := listRendering{
		Ident: identifier,
		Rows:  values,
	}

	// TODO: Template caching plesae
	t, _ := template.ParseFiles("list.html")
	t.Execute(w, p)
}

func handler(w http.ResponseWriter, r *http.Request) {
	// Figure out if there's a path to the request
	// No path = index
	// POST = store
	// GET with ?store as parameter = store
	// GET = list
	// no Cors needed! lol!

	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[0]

	if r.Method == "GET" {
		log.Println("GETTING!")
		listItems(w, path)
	} else if r.Method == "POST" {
		log.Println("POSTING!")
		bd, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Couldn't read body from POST request")
			w.WriteHeader(400)
			w.Write([]byte("No thanks"))
		}
		store(w, path, string(bd[:]))
	} else {
		w.WriteHeader(400)
		w.Write([]byte("Method not supported"))
	}
}

func main() {
	log.Println("[+] Starting www gathering thingamadoo")
	dsn := flag.String("dsn", "postgres://localhost/store", "a valid database DSN to store the data in")
	cert := flag.String("cert", "./cert.pem", "valid TLS certificate for hosting https")
	key := flag.String("key", "./key.pem", "valid TLS Private key for hosting https")
	port := flag.Int("port", 443, "the port to host on. Advised to host on 443 due to normal https firewall rules")
	host := flag.String("host", "", "the host to listen on, leave empty for all interfaces.")
	flag.Parse()

	var err error

	// Input sanity checking
	if *port < 0 || *port > 65535 {
		log.Fatal("Cannot use strange port numbers, behave")
	}

	if _, err := os.Stat(*cert); err != nil {
		log.Fatal("Certificate file doesn't exist", err)
	}

	if _, err := os.Stat(*key); err != nil {
		log.Fatal("Key file doesn't exist", err)
	}

	databaseClient, err = sql.Open("postgres", *dsn)
	if err != nil {
		panic(err)
	}
	defer databaseClient.Close()
	err = databaseClient.Ping()
	if err != nil {
		panic(err)
	}

	hostname := fmt.Sprintf("%s:%d", *host, *port)

	log.Println("[+] Database connected")
	log.Println("[+] Starting listener")
	http.HandleFunc("/", handler)
	err = http.ListenAndServeTLS(hostname, *cert, *key, nil)
	// Since err is always non-nil, no need to do special things here I
	// guess
	log.Println(err)
}

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
func store(w http.ResponseWriter, identifier string, data string, remote string) {
	q := "insert into items (ident, ts, data, remote) VALUES ($1, $2, $3, $4)"

	_, err := databaseClient.Query(q, identifier, time.Now(), data, remote)
	if err != nil {
		log.Fatalln(err)
	}

	w.Write([]byte("Thanks!"))
}

type renderItem struct {
	Value      string
	RemoteAddr string
}

type listRendering struct {
	Ident string
	Rows  []renderItem
}

// lists all stored items under the identifier
func listItems(w http.ResponseWriter, identifier string) {
	rows, err := databaseClient.Query("select data, remote from items where ident = $1 order by ts desc", identifier)
	if err != nil {
		w.WriteHeader(500)
		log.Println(err)
		return
	}
	defer rows.Close()

	var values []renderItem

	for rows.Next() {
		var data string
		var remote string
		err := rows.Scan(&data, &remote)
		if err != nil {
			continue
		}

		values = append(values, renderItem{
			Value:      data,
			RemoteAddr: remote})
	}

	p := listRendering{
		Ident: identifier,
		Rows:  values,
	}

	// TODO: Template caching plesae
	t, _ := template.ParseFiles("list.html")
	t.Execute(w, p)
}

func getRemoteAddr(r *http.Request) string {
	remoteAddr := r.RemoteAddr
	if forwarded, ok := r.Header["X-Forwarded-For"]; ok {
		return strings.Join(forwarded, " ")
	}
	return remoteAddr
}

func handler(w http.ResponseWriter, r *http.Request) {
	path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[0]

	log.Println(r.Method, r.URL.Path, "-", getRemoteAddr(r), "-", r.ContentLength)
	if r.Method == "GET" {
		listItems(w, path)
	} else if r.Method == "POST" {
		bd, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("Couldn't read body from POST request")
			w.WriteHeader(400)
			w.Write([]byte("No thanks"))
		}
		store(w, path, string(bd[:]), getRemoteAddr(r))
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

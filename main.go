package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq" // postgres driver for database/sql
)

const psqlInfo = "host=localhost port=5432 user=default password=default dbname=default sslmode=disable"

func main() {
	http.HandleFunc("POST /login", login)
	http.HandleFunc("POST /user/register", register)
	http.HandleFunc("GET /user", get_user)
	http.ListenAndServe(":8080", nil)
}

func login(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, r.Method)
	q := r.URL.Query()
	name := q.Get("name")
	fmt.Fprintf(w, "Hello, %s!", name)
}
func register(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(w, r.Method)
	// fmt.Fprintf(w, r.URL.Path)
	// fmt.Fprintf(w, r.URL.RawQuery)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	q := r.URL.Query()
	name := q.Get("name")

	stmt, err := db.Prepare("INSERT INTO users(name) VALUES($1)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(name)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close() // close the connection

	fmt.Fprintln(w, res)
}
func get_user(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, r.Method)
	fmt.Fprintf(w, r.URL.Path)
	fmt.Fprintf(w, r.URL.RawQuery)
}

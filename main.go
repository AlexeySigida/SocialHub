package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	_ "github.com/lib/pq" // postgres driver for database/sql
)

const psqlInfo = "host=localhost port=5432 user=default password=default dbname=default sslmode=disable"

type User struct {
	first_name  string `json: first_name`
	second_name string `json: second_name`
	birthdate   string `json: birthdate`
	sex         string `json: sex`
	biography   string `json: biography`
	city        string `json: city`
	username    string `json: username`
	password    string `json: password`
}

// sha256Hash возвращает SHA-256 хэш строки.
func sha256StringHash(input string) string {
	// Используем sha256.New для создания хэш-функции.
	hash := sha256.New()
	// Пишем строку в хэш-функцию.
	hash.Write([]byte(input))
	// Возвращаем хэш в виде строки.
	return hex.EncodeToString(hash.Sum(nil))
}

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

	var user User

	decoder := json.NewDecoder(r.Body)
	err_decoding := decoder.Decode(&user)
	fmt.Println(user.first_name, user.second_name)

	if err_decoding != nil {
		fmt.Println("Error decoding JSON")
		return
	}

	stmt, err := db.Prepare("INSERT INTO public.users" +
		"(first_name, second_name, birthdate, sex, biography, city, username, password)" +
		" VALUES($1,$2,$3,$4,$5,$6,$7,$8")
	if err != nil {
		fmt.Println(err.Error())
	}
	defer stmt.Close()

	dateParse, err := time.Parse("2006-01-02", user.birthdate)

	res, err := stmt.Exec(user.first_name,
		user.second_name,
		dateParse,
		user.sex,
		user.biography,
		user.city,
		user.username,
		sha256StringHash(user.password))
	if err != nil {
		fmt.Println(err.Error())
	}
	defer db.Close() // close the connection

	fmt.Fprintln(w, res)
}

func get_user(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, r.Method)
	fmt.Fprintf(w, r.URL.Path)
	fmt.Fprintf(w, r.URL.RawQuery)
}

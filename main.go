package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	_ "github.com/lib/pq" // postgres driver for database/sql
)

const psqlInfo = "host=db port=5432 user=default password=default dbname=default sslmode=disable"

type User struct {
	FirstName  string `json:"first_name"`
	SecondName string `json:"second_name"`
	Birthdate  string `json:"birthdate"`
	Sex        string `json:"sex"`
	Biography  string `json:"biography"`
	City       string `json:"city"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var jwtKey = []byte("secret")
var tokens []string

type Token struct {
	Token string `json:"token"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
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

func generateJWT() (string, error) {
	expirationTime := time.Now().Add(5 * time.Minute)
	claims := &Claims{
		Username: "username",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(jwtKey)

}

// Функция для чтения всех параметров запроса и возврата их в виде словаря
func readQueryParams(r *http.Request) map[string][]string {
	params := make(map[string][]string)
	q := r.URL.Query()
	for key, values := range q {
		params[key] = values
	}
	return params
}

func main() {
	http.HandleFunc("POST /login", login)
	http.HandleFunc("POST /user/register", register)
	http.HandleFunc("GET /user", get_user)
	http.HandleFunc("GET /user/search", search_like_fname_sname)
	http.ListenAndServe(":8080", nil)
}

func login(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	var user Auth

	b, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	err_decoding := json.Unmarshal(b, &user)

	if err_decoding != nil {
		http.Error(w, err_decoding.Error(), http.StatusBadRequest)
	} else {
		stmt, err := db.Prepare("SELECT username, password FROM public.users WHERE username=$1")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		defer stmt.Close()

		rows, err := stmt.Query(user.Username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			for rows.Next() {
				var (
					username string
					password string
				)
				if err := rows.Scan(&username, &password); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
				} else {
					if user.Username == username && sha256StringHash(user.Password) == password {
						token, _ := generateJWT()
						tokens = append(tokens, token)

						tok := Token{token}
						token_json, _ := json.Marshal(tok)

						fmt.Fprintln(w, string(token_json))
					} else {
						http.Error(w, "Unauthorized", http.StatusUnauthorized)
					}
				}
			}
		}
	}

}

func register(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	var user User

	b, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	err_decoding := json.Unmarshal(b, &user)

	if err_decoding != nil {
		http.Error(w, err_decoding.Error(), http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("INSERT INTO public.users" +
		"(id,first_name, second_name, birthdate, sex, biography, city, username, password)" +
		" VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	defer stmt.Close()

	dateParse, err_date_parsing := time.Parse("2006-01-02", user.Birthdate)

	if err_date_parsing != nil {
		http.Error(w, err_date_parsing.Error(), http.StatusBadRequest)
	}
	id := uuid.New()
	res, err := stmt.Exec(id.String(), user.FirstName,
		user.SecondName,
		dateParse,
		user.Sex,
		user.Biography,
		user.City,
		user.Username,
		sha256StringHash(user.Password))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else if res != nil {
		fmt.Fprintln(w, id.String())
	}
	defer db.Close() // close the connection
}

func get_user(w http.ResponseWriter, r *http.Request) {
	bearerToken := r.Header.Get("Authorization")
	reqToken := strings.Split(bearerToken, " ")[1]
	claims := &Claims{}
	tkn, err := jwt.ParseWithClaims(reqToken, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !tkn.Valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	defer db.Close()

	type Params struct {
		Id string `json:"id"`
	}

	var params Params

	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	err_decoding := json.Unmarshal(b, &params)

	if err_decoding != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		stmt, err := db.Prepare("SELECT first_name,second_name,birthdate,sex,biography,city FROM public.users WHERE id=$1")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		defer stmt.Close()

		rows, err := stmt.Query(params.Id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			var users []User
			for rows.Next() {
				var (
					FirstName  string
					SecondName string
					Birthdate  string
					Sex        string
					Biography  string
					City       string
				)
				if err := rows.Scan(&FirstName, &SecondName, &Birthdate, &Sex, &Biography, &City); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
				} else {
					user := User{FirstName, SecondName, Birthdate, Sex, Biography, City, "", ""}
					users = append(users, user)
				}
			}
			resp, _ := json.Marshal(users)
			fmt.Fprint(w, string(resp))
		}
	}
}

func search_like_fname_sname(w http.ResponseWriter, r *http.Request) {
	bearerToken := r.Header.Get("Authorization")
	reqToken := strings.Split(bearerToken, " ")[1]
	claims := &Claims{}
	tkn, err := jwt.ParseWithClaims(reqToken, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !tkn.Valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	defer db.Close()

	type Params struct {
		Fname string `json:"first_name"`
		Sname string `json:"second_name"`
	}

	var params Params

	for key, values := range readQueryParams(r) {
		if key == "first_name" {
			params.Fname = values[0]
		} else if key == "second_name" {
			params.Sname = values[0]
		}
	}

	stmt, err := db.Prepare("SELECT first_name,second_name,birthdate,sex,biography,city FROM public.users WHERE first_name LIKE $1 AND second_name LIKE $2")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	defer stmt.Close()

	rows, err := stmt.Query(fmt.Sprintf("%%%s%%", params.Fname), fmt.Sprintf("%%%s%%", params.Sname))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		var users []User
		for rows.Next() {
			var (
				FirstName  string
				SecondName string
				Birthdate  string
				Sex        string
				Biography  string
				City       string
			)
			if err := rows.Scan(&FirstName, &SecondName, &Birthdate, &Sex, &Biography, &City); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				user := User{FirstName, SecondName, Birthdate, Sex, Biography, City, "", ""}
				users = append(users, user)
			}
		}
		resp, _ := json.Marshal(users)
		fmt.Fprint(w, string(resp))
	}
}

package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	_ "github.com/lib/pq" // postgres driver for database/sql
)

const psqlMasterInfo = "host=db port=5432 user=postgres password=pass dbname=postgres sslmode=disable"
const psqlDialogInfo = "host=citus_master port=5432 user=postgres password=pass dbname=postgres sslmode=disable"

var rdb *redis.Client
var ctx = context.Background()

// const psqlSlaveInfo = "host=slave1 port=5432 user=postgres password=pass dbname=postgres sslmode=disable"

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

type AuthorzatedUser struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

var authorizatedUsers []AuthorzatedUser

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type Post struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
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

func generateJWT(username string) (string, error) {
	expirationTime := time.Now().Add(60 * time.Minute)
	claims := &Claims{
		Username: username,
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

func initializeCash() error {
	db, err := sql.Open("postgres", psqlMasterInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	posts, err := rdb.LRange(ctx, "posts_feed", 0, -1).Result()
	if err != nil {
		return fmt.Errorf("error in getting: %v", err)
	}
	if len(posts) > 10 {
		return nil
	}

	rows, err := db.Query("SELECT id, content from public.posts limit 10")
	if err != nil {
		return fmt.Errorf("error in query: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var post Post
		if err := rows.Scan(&post.ID, &post.Content); err != nil {
			return fmt.Errorf("error in scan: %v", err)
		}

		postData, err := json.Marshal(post)
		if err != nil {
			return fmt.Errorf("error in marshal: %v", err)
		}

		fmt.Println(string(postData))

		if err := rdb.RPush(ctx, "posts_feed", postData).Err(); err != nil {
			return fmt.Errorf("error in push: %v", err)
		}
	}

	return nil
}

func initializeDialog() error {
	db, err := sql.Open("postgres", psqlDialogInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err1 := db.Query("SELECT * from public.dialog limit 1")
	if err1 != nil {
		_, err2 := db.Query("CREATE TABLE dialog (sender_id VARCHAR(255) NOT NULL, getter_id VARCHAR(255) NOT NULL, message text NOT NULL, message_dt timestamp NOT NULL);")
		if err2 != nil {
			return fmt.Errorf("error in query: %v", err1)
		}
		_, err3 := db.Query("SELECT create_distributed_table('dialog', 'message_dt');")
		if err3 != nil {
			return fmt.Errorf("error in query: %v", err2)
		}
	}

	return nil
}

func main() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	if err := initializeCash(); err != nil {
		panic(err)
	}

	if err := initializeDialog(); err != nil {
		panic(err)
	}

	http.HandleFunc("POST /login", login)
	http.HandleFunc("POST /user/register", register)
	http.HandleFunc("GET /user", get_user)
	http.HandleFunc("GET /user/search", search_like_fname_sname)
	http.HandleFunc("GET /post/feed", post_feed)
	http.HandleFunc("POST /post/add", post_add)
	http.HandleFunc("POST /dialog/send", dialog_send)
	http.HandleFunc("GET /dialog/list", dialog_list)

	http.ListenAndServe(":8080", nil)
}

func login(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("postgres", psqlMasterInfo)
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
						token, _ := generateJWT(username)

						tokens = append(tokens, token)

						tok := Token{token}

						token_json, _ := json.Marshal(tok)

						authorizatedUsers = append(authorizatedUsers, AuthorzatedUser{username, tok.Token})

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
	db, err := sql.Open("postgres", psqlMasterInfo)
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

	db, err := sql.Open("postgres", psqlMasterInfo)
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

	db, err := sql.Open("postgres", psqlMasterInfo)
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

	stmt, err := db.Prepare("SELECT first_name,second_name FROM public.users WHERE first_name LIKE $1 AND second_name LIKE $2")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	defer stmt.Close()

	rows, err := stmt.Query(fmt.Sprintf("%s%%", params.Fname), fmt.Sprintf("%s%%", params.Sname))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		var users []User
		for rows.Next() {
			var (
				FirstName  string
				SecondName string
			)
			if err := rows.Scan(&FirstName, &SecondName); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			} else {
				user := User{FirstName, SecondName, "", "", "", "", "", ""}
				users = append(users, user)
			}
		}
		resp, _ := json.Marshal(users)
		fmt.Fprint(w, string(resp))
	}
}

func post_feed(w http.ResponseWriter, r *http.Request) {
	posts, err := rdb.LRange(ctx, "posts_feed", 0, -1).Result()
	if err != nil {
		http.Error(w, "Error getting posts from Redis", http.StatusInternalServerError)
		return
	}

	var feed []Post
	for _, post := range posts {
		var p Post
		if err := json.Unmarshal([]byte(post), &p); err != nil {
			http.Error(w, "Error unmarshalling post", http.StatusInternalServerError)
			fmt.Println(post)
			return
		}
		feed = append(feed, p)
	}

	resp, _ := json.Marshal(feed)
	fmt.Fprint(w, string(resp))
}

func post_add(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("postgres", psqlMasterInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	var post Post
	if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	postData, err := json.Marshal(post)
	if err != nil {
		http.Error(w, "Error marshalling post", http.StatusInternalServerError)
		return
	}

	if err := rdb.RPush(ctx, "posts_feed", postData).Err(); err != nil {
		http.Error(w, "Error adding post to Redis", http.StatusInternalServerError)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Error starting transaction", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec("insert into public.posts(id,user_id,content,post_date) values ($1,'12346',$2 ,timestamp '2024-07-29')", post.ID, post.Content)
	if err != nil {
		tx.Rollback()
		http.Error(w, "Error adding post to database", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Error committing transaction", http.StatusInternalServerError)
		return
	}

	posts, err := rdb.LRange(ctx, "posts_feed", 0, -1).Result()
	if err != nil {
		return
	}
	if len(posts) > 10 {
		if err := rdb.LPop(ctx, "posts_feed").Err(); err != nil {
			// http.Error(w, "Error deleting posts from Redis", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func dialog_send(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("postgres", psqlDialogInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db_master, err := sql.Open("postgres", psqlMasterInfo)
	if err != nil {
		panic(err)
	}
	defer db_master.Close()

	type Dialog struct {
		GetterId string `json:"getter_id"`
		Text     string `json:"text"`
	}

	var dialog Dialog
	if err := json.NewDecoder(r.Body).Decode(&dialog); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var current_user string
	token := r.Header.Get("Authorization")
	for i := 0; i < len(authorizatedUsers); i++ {
		if strings.Replace(token, "Bearer ", "", -1) == authorizatedUsers[i].Token {
			current_user = authorizatedUsers[i].Username
			break
		}
	}
	if current_user == "" {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	rows, err2 := db_master.Query("select id from public.users where username=$1 limit 1", current_user)
	if err2 != nil {
		http.Error(w, "Error getting user id", http.StatusInternalServerError)
		return
	}
	for rows.Next() {
		if err := rows.Scan(&current_user); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}

	_, err1 := db.Query("insert into public.dialog(sender_id,getter_id,message,message_dt) values ($1,$2,$3,$4)", current_user, dialog.GetterId, dialog.Text, time.Now())
	if err1 != nil {
		fmt.Println(err1)
		http.Error(w, "Error adding dialog to database", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Sent")
}

func dialog_list(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("postgres", psqlDialogInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	db_master, err := sql.Open("postgres", psqlMasterInfo)
	if err != nil {
		panic(err)
	}
	defer db_master.Close()

	var current_user string

	token := r.Header.Get("Authorization")
	for i := 0; i < len(authorizatedUsers); i++ {
		if strings.Replace(token, "Bearer ", "", -1) == authorizatedUsers[i].Token {
			current_user = authorizatedUsers[i].Username
			break
		}
	}
	if current_user == "" {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	rows, err2 := db_master.Query("select id from public.users where username=$1 limit 1", current_user)
	if err2 != nil {
		http.Error(w, "Error getting user id", http.StatusInternalServerError)
		return
	}
	for rows.Next() {
		if err := rows.Scan(&current_user); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}

	reciepient_id := r.URL.Query().Get("reciepient_id")
	if reciepient_id == "" {
		http.Error(w, "reciepient_id is required", http.StatusBadRequest)
		return
	}

	rows, err1 := db.Query("select sender_id as from, getter_id as to, message as text from public.dialog where sender_id=$1 and getter_id=$2 or sender_id=$2 and getter_id=$1 order by message_dt desc", current_user, reciepient_id)
	if err1 != nil {
		http.Error(w, "Error getting dialogs", http.StatusInternalServerError)
		return
	}

	type Dialog struct {
		From string `json:"from"`
		To   string `json:"to"`
		Text string `json:"text"`
	}
	var dialog []Dialog
	for rows.Next() {
		var (
			From string
			To   string
			Text string
		)
		if err := rows.Scan(&From, &To, &Text); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			dialog = append(dialog, Dialog{From: From, To: To, Text: Text})
		}
	}

	resp, _ := json.Marshal(dialog)
	fmt.Fprint(w, string(resp))
}

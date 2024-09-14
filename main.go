package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/streadway/amqp"
	"github.com/tarantool/go-tarantool"

	_ "github.com/lib/pq" // postgres driver for database/sql
)

const psqlMasterInfo = "host=haproxy port=5432 user=postgres password=pass dbname=postgres sslmode=disable"
const psqlDialogInfo = "host=citus_master port=5432 user=postgres password=pass dbname=postgres sslmode=disable"

var rdb *redis.Client
var ctx = context.Background()

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var rabbitConn *amqp.Connection

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
	conn, err := tarantool.Connect("tarantool:3301", tarantool.Opts{
		User: "guest",
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Call("initialize_dialog", []interface{}{})
	if err != nil {
		return fmt.Errorf("error in initialize_dialog: %v", err)
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
	var err error
	rabbitConn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		panic(err)
	}
	defer rabbitConn.Close()

	router := mux.NewRouter()

	router.HandleFunc("/dialog/send", proxyDialogSend).Methods("POST")
	router.HandleFunc("/dialog/list", proxyDialogList).Methods("GET")
	router.HandleFunc("/login", login).Methods("POST")
	router.HandleFunc("/user/register", register).Methods("POST")
	router.HandleFunc("/user", get_user).Methods("GET")
	router.HandleFunc("/user/search", search_like_fname_sname).Methods("GET")
	router.HandleFunc("/post/feed", post_feed).Methods("GET")
	router.HandleFunc("/post/add", post_add).Methods("POST")
	router.HandleFunc("/post/create", post_create)
	router.HandleFunc("/post/feed/posted", post_feed_posted)

	log.Fatal(http.ListenAndServe(":8080", router))
}

// Proxy to forward the request to the chat service
func proxyDialogSend(w http.ResponseWriter, r *http.Request) {
	proxyRequest(w, r, "http://chat-service:8081/dialog/send")
}

func proxyDialogList(w http.ResponseWriter, r *http.Request) {
	// Get the original query parameters from the source request
	queryParams := r.URL.RawQuery

	// Define the base URL for the chat service
	baseURL := "http://chat-service:8081/dialog/list"

	// If there are query parameters, append them to the proxy URL
	var proxyURL string
	if queryParams != "" {
		proxyURL = fmt.Sprintf("%s?%s", baseURL, queryParams)
	} else {
		proxyURL = baseURL
	}

	// Proxy the request to the chat service with the constructed URL
	proxyRequest(w, r, proxyURL)
}

// A generic function to proxy the request
func proxyRequest(w http.ResponseWriter, r *http.Request, url string) {
	req, err := http.NewRequest(r.Method, url, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("X-User-Id", getCurrentUser(r.Header.Get("Authorization")))
	req.Header.Set("X-Request-Id", r.Header.Get("X-Request-Id"))

	client := &http.Client{Timeout: 10 * time.Second}
	fmt.Println(req)
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to call chat service", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read chat service response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
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

						var current_user string

						rows, err2 := db.Query("select id from public.users where username=$1 limit 1", username)
						if err2 != nil {
							panic(err2)
						}
						for rows.Next() {
							if err := rows.Scan(&current_user); err != nil {
								panic(err)
							}
						}

						stmt, err := db.Prepare("INSERT INTO public.tokens" +
							"(user_id, token, created_at)" +
							" VALUES($1,$2,$3)")
						if err != nil {
							http.Error(w, err.Error(), http.StatusBadRequest)
						}
						defer stmt.Close()
						_, errInsert := stmt.Exec(current_user, token, time.Now())
						if errInsert != nil {
							http.Error(w, err.Error(), http.StatusBadRequest)
						}
						defer db.Close() // close the connection

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
	conn, err := tarantool.Connect("tarantool:3301", tarantool.Opts{
		User: "guest",
	})
	if err != nil {
		http.Error(w, "Connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	var dialog struct {
		GetterId string `json:"getter_id"`
		Text     string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&dialog); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Assuming token and current_user logic remains the same
	token := r.Header.Get("Authorization")
	current_user := getCurrentUser(token)

	_, err = conn.Call("dialog_send", []interface{}{current_user, dialog.GetterId, dialog.Text})
	if err != nil {
		http.Error(w, "Error adding dialog to database", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Sent")
}

func convertMapI2S(data interface{}) interface{} {
	switch v := data.(type) {
	case map[interface{}]interface{}:
		newMap := make(map[string]interface{})
		for key, value := range v {
			strKey := fmt.Sprintf("%v", key)      // Convert key to string
			newMap[strKey] = convertMapI2S(value) // Recursively convert values
		}
		return newMap
	case []interface{}:
		for i, value := range v {
			v[i] = convertMapI2S(value) // Recursively convert elements in slices
		}
	}
	return data
}

func dialog_list(w http.ResponseWriter, r *http.Request) {
	conn, err := tarantool.Connect("tarantool:3301", tarantool.Opts{
		User: "guest",
	})
	if err != nil {
		http.Error(w, "Connection error", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	token := r.Header.Get("Authorization")
	current_user := getCurrentUser(token)

	recipient_id := r.URL.Query().Get("reciepient_id")
	if recipient_id == "" {
		http.Error(w, "reciepient_id is required", http.StatusBadRequest)
		return
	}

	resp, err := conn.Call("dialog_list", []interface{}{current_user, recipient_id})
	if err != nil {
		http.Error(w, "Error getting dialogs", http.StatusInternalServerError)
		return
	}

	fmt.Println(resp)
	fmt.Println(resp.Data[0])
	fmt.Println(reflect.TypeOf(resp))
	fmt.Println(reflect.TypeOf(resp.Data[0]))
	fmt.Println(convertMapI2S(resp.Data))
	dialogs, err := json.Marshal(convertMapI2S(resp.Data))
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Error encoding dialogs", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(dialogs))
}

func post_create(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error init upgrader", err)
		return
	}
	defer ws.Close()
	db_master, err := sql.Open("postgres", psqlMasterInfo)
	if err != nil {
		log.Println("Error init postgres conn", err)
		return
	}
	defer db_master.Close()

	var post Post
	if err := ws.ReadJSON(&post); err != nil {
		log.Println("Invalid request body", err)
		return
	}

	tx, err := db_master.Begin()
	if err != nil {
		log.Println("Error starting transaction", err)
		return
	}

	_, err = tx.Exec("insert into public.posts(id,user_id,content,post_date) values ($1,'12346',$2 ,timestamp '2024-07-29')", post.ID, post.Content)
	if err != nil {
		tx.Rollback()
		log.Println("Error adding post to database", err)
		return
	}

	if err = tx.Commit(); err != nil {
		log.Println("Error committing transaction", err)
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
		log.Println("Invalid token", err)
		return
	}
	rows, err2 := db_master.Query("select id from public.users where username=$1 limit 1", current_user)
	if err2 != nil {
		log.Println("Error getting user id", err)
		return
	}
	for rows.Next() {
		if err := rows.Scan(&current_user); err != nil {
			log.Println(err.Error())
		}
	}

	publishPost(post, current_user)
}

func post_feed_posted(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close()

	db, err := sql.Open("postgres", psqlMasterInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	var current_user string

	token := r.Header.Get("Authorization")
	for i := 0; i < len(authorizatedUsers); i++ {
		if strings.Replace(token, "Bearer ", "", -1) == authorizatedUsers[i].Token {
			current_user = authorizatedUsers[i].Username
			break
		}
	}
	if current_user == "" {
		log.Println("Invalid token", err)
		return
	}
	rows, err2 := db.Query("select id from public.users where username=$1 limit 1", current_user)
	if err2 != nil {
		log.Println("Error getting user id", err)
		return
	}
	for rows.Next() {
		if err := rows.Scan(&current_user); err != nil {
			log.Println(err.Error())
		}
	}

	ch, err := rabbitConn.Channel()
	if err != nil {
		log.Println(err)
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		current_user,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println(err)
		return
	}

	msgs, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Println(err)
		return
	}

	for msg := range msgs {
		var post Post
		err := json.Unmarshal(msg.Body, &post)
		if err != nil {
			log.Println(err)
			continue
		}

		ws.WriteJSON(post)
	}
}

func publishPost(post Post, user_id string) {
	ch, err := rabbitConn.Channel()
	if err != nil {
		log.Println(err)
		return
	}
	defer ch.Close()

	db, err := sql.Open("postgres", psqlMasterInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rows, err := db.Query("select user_id from public.subscribers where user_id=$1", user_id)
	if err != nil {
		log.Println(err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var subscriber_id string
		err := rows.Scan(&subscriber_id)
		if err != nil {
			log.Println(err)
			continue
		}
		body, err := json.Marshal(post)
		if err != nil {
			log.Println(err)
			continue
		}

		err = ch.Publish(
			"",
			subscriber_id,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
			},
		)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

func getCurrentUser(token string) string {
	var current_user string
	db_master, err := sql.Open("postgres", psqlMasterInfo)
	if err != nil {
		panic(err)
	}
	defer db_master.Close()

	rowsToken, errToken := db_master.Query("select user_id from public.tokens where token=$1 order by created_at desc limit 1", strings.Replace(token, "Bearer ", "", -1))
	if errToken != nil {
		panic(errToken)
	}
	for rowsToken.Next() {
		if err := rowsToken.Scan(&current_user); err != nil {
			panic(err)
		}
	}

	if current_user == "" {
		panic("Empty user")
	}

	rowsUser, errUser := db_master.Query("select id from public.users where username=$1 limit 1", current_user)
	if errUser != nil {
		panic(errUser)
	}

	for rowsUser.Next() {
		if err := rowsUser.Scan(&current_user); err != nil {
			panic(err)
		}
	}

	return current_user
}

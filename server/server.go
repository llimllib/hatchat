package server

// TODO:
// - user new muxer to verify proper HTTP methods
// - logging middleware
// - log when listening

import (
	"context"
	"log"
	"net/http"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

var dbpool *sqlitex.Pool

type ChatServer struct {
	db *sqlitex.Pool
}

func initDB() *sqlitex.Pool {
	var err error
	dbpool, err = sqlitex.Open("file:chat.db", 0, 10)
	if err != nil {
		log.Fatal(err)
	}
	conn := dbpool.Get(context.Background())
	if conn == nil {
		log.Fatal("unable to get connection")
	}
	if err := sqlitex.Exec(conn, "CREATE TABLE IF NOT EXISTS users(id int primary key not null, username text, password text)", nil); err != nil {
		log.Fatalf("unable to create users table %s", err)
	}
	return dbpool
}

func serveChat(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	http.ServeFile(w, r, "template/chat.html")
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	http.ServeFile(w, r, "template/home.html")
}

func (h *ChatServer) register(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, "template/register.html")
		return
	}
	conn := h.db.Get(context.Background())
	if conn == nil {
		return
	}
	if err := sqlitex.Exec(conn, "INSERT INTO users(username, password) FROM users WHERE username=? AND password=?", nil, r.FormValue("username"), r.FormValue("password")); err != nil {
		log.Fatalf("insert error: %v\n", err)
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *ChatServer) login(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, "template/login.html")
		return
	}
	conn := h.db.Get(context.Background())
	if conn == nil {
		return
	}

	userID := int64(-1)
	if err := sqlitex.Exec(conn, "SELECT (id) FROM users WHERE username=? AND password=?", func(stmt *sqlite.Stmt) error {
		userID = stmt.ColumnInt64(0)
		return nil
	}, r.FormValue("username"), r.FormValue("password")); err != nil {
		log.Fatalf("query error: %v\n", err)
	}
	log.Printf("params: %s %s\n", r.FormValue("username"), r.FormValue("password"))
	if userID == -1 {
		log.Println("login failed")
	} else {
		log.Println("login succeeded")
		http.Redirect(w, r, "/chat", http.StatusFound)
	}
}

func NewChatServer() *ChatServer {
	db := initDB()
	return &ChatServer{db}
}

func (h *ChatServer) Run(addr string) {
	hub := newHub()
	go hub.run()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/chat", serveChat)
	http.HandleFunc("/register", h.register)
	http.HandleFunc("/login", h.login)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 3 * time.Second,
	}
	log.Printf("listening on %s\n", addr)
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

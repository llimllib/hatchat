package server

// TODO:
// - user new muxer to verify proper HTTP methods
// - logging middleware
// - log when listening

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"github.com/lmittmann/tint"

	"github.com/llimllib/tinychat/server/middleware"
)

var dbpool *sqlitex.Pool

type ChatServer struct {
	db     *sqlitex.Pool
	logger *slog.Logger
}

func fatal(logger *slog.Logger, message string, err error, args ...any) {
	args = append(args, "error")
	args = append(args, err)
	logger.Error(message, args...)
	panic(message)
}

func initDB(logger *slog.Logger) *sqlitex.Pool {
	var err error
	dbpool, err = sqlitex.Open("file:chat.db", 0, 10)
	if err != nil {
		fatal(logger, "Unable to open db", err)
	}
	conn := dbpool.Get(context.Background())
	if conn == nil {
		fatal(logger, "unable to get connection", nil)
	}
	if err := sqlitex.Exec(conn, "CREATE TABLE IF NOT EXISTS users(id int primary key not null, username text, password text)", nil); err != nil {
		fatal(logger, "unable to create users table", err)
	}
	return dbpool
}

func (h *ChatServer) serveChat(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "template/chat.html")
}

func (h *ChatServer) serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "template/home.html")
}

func (h *ChatServer) register(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, "template/register.html")
		return
	}
	conn := h.db.Get(context.Background())
	if conn == nil {
		return
	}
	if err := sqlitex.Exec(conn, "INSERT INTO users(username, password) FROM users WHERE username=? AND password=?", nil, r.FormValue("username"), r.FormValue("password")); err != nil {
		fatal(h.logger, "insert error", err)
	}
	h.logger.Debug("inserted user", "username", r.FormValue("username"))
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *ChatServer) login(w http.ResponseWriter, r *http.Request) {
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
		fatal(h.logger, "query error", err)
	}
	if userID == -1 {
		h.logger.Debug("login failed")
	} else {
		h.logger.Debug("login succeeded")
		http.Redirect(w, r, "/chat", http.StatusFound)
	}
}

func env(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

// create a logger with its log level based on the LOG_LEVEL environment var,
// defaulting to INFO
func initLog() *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(env("LOG_LEVEL", "INFO"))); err != nil {
		fatal(slog.Default(), "Unable to convert log level", err)
	}
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level: level,
	}))
	logger.Debug("started logger", "level", level)
	return logger
}

func NewChatServer() *ChatServer {
	logger := initLog()
	db := initDB(logger)
	return &ChatServer{db, logger}
}

func (h *ChatServer) Run(addr string) {
	h.logger.Info("Starting server", "addr", addr)
	hub := newHub()
	go hub.run()

	requestID := middleware.RequestIDMiddleware(h.logger)
	logReq := middleware.RequestLogMiddleware(h.logger)
	http.HandleFunc("/", requestID(logReq(h.serveHome)))
	http.HandleFunc("/chat", requestID(logReq(h.serveChat)))
	http.HandleFunc("/register", requestID(logReq(h.register)))
	http.HandleFunc("/login", requestID(logReq(h.login)))
	http.HandleFunc("/ws", requestID(logReq(func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})))
	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 3 * time.Second,
	}
	h.logger.Info("listening", "addr", addr)
	err := server.ListenAndServe()
	if err != nil {
		fatal(h.logger, "ListenAndServe", err)
	}
}

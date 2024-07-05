package server

// TODO:
// - user new muxer to verify proper HTTP methods
// - logging middleware
// - log when listening

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"github.com/google/uuid"
	"github.com/lmittmann/tint"
	"golang.org/x/crypto/bcrypt"

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
	if r.Method != http.MethodPost {
		h.logger.Debug("wrong method")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	conn := h.db.Get(context.Background())
	if conn == nil {
		return
	}

	// TODO: add a message (where?) to display as a toast
	user := r.FormValue("username")
	if user == "" {
		h.logger.Debug("missing username")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	pass := r.FormValue("password")
	if pass == "" {
		h.logger.Debug("missing password")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	encPass, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Debug("unable to encrypt pass")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	uid := fmt.Sprintf("usr_%s", uuid.New())

	if err := sqlitex.Exec(conn, "INSERT INTO users(id, username, password) VALUES(?, ?, ?)", nil, uid, user, encPass); err != nil {
		fatal(h.logger, "insert error", err)
	}
	h.logger.Debug("inserted user", "username", r.FormValue("username"))
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *ChatServer) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Debug("wrong method")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	conn := h.db.Get(context.Background())
	if conn == nil {
		return
	}

	user := r.FormValue("username")
	if user == "" {
		h.logger.Debug("missing username")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	pass := r.FormValue("password")
	if pass == "" {
		h.logger.Debug("missing password")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	userID := int64(-1)
	hashedPass := ""
	if err := sqlitex.Exec(conn, "SELECT id, password FROM users WHERE username=?", func(stmt *sqlite.Stmt) error {
		h.logger.Debug("here")
		userID = stmt.ColumnInt64(0)
		h.logger.Debug("", "userID", userID)
		hashedPass = stmt.ColumnText(1)
		h.logger.Debug("", "hashedPass", hashedPass)
		return nil
	}, user); err != nil {
		fatal(h.logger, "query error", err)
	}

	if userID == -1 {
		h.logger.Debug("login failed")
	} else {
		if err := bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(pass)); err == nil {
			h.logger.Debug("login succeeded")
			http.Redirect(w, r, "/chat", http.StatusFound)
		} else {
			h.logger.Debug("wrong password")
			http.Redirect(w, r, "/", http.StatusFound)
		}
	}
}

func (h *ChatServer) serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("path", "path", r.URL.Path)
	if _, err := filepath.Abs(r.URL.Path); err == nil {
		http.ServeFile(w, r, r.URL.Path)
	} else {
		http.NotFound(w, r)
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

func (h *ChatServer) middleware(route string, handler http.HandlerFunc) http.HandlerFunc {
	requestID := middleware.RequestIDMiddleware(h.logger)
	logReq := middleware.RequestLogMiddleware(h.logger)(route)
	panicHandler := middleware.RecoverMiddleware(h.logger)
	return panicHandler(requestID(logReq(handler)))
}

func (h *ChatServer) Run(addr string) {
	h.logger.Info("Starting server", "addr", addr)
	hub := newHub()
	go hub.run()

	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))).ServeHTTP
	http.HandleFunc("/static/", h.middleware("/static", staticHandler))
	http.HandleFunc("/chat", h.middleware("/chat", h.serveChat))
	http.HandleFunc("/register", h.middleware("/register", h.register))
	http.HandleFunc("/login", h.middleware("/login", h.login))
	http.HandleFunc("/ws", h.middleware("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	}))
	http.HandleFunc("/", h.middleware("/", h.serveHome))

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

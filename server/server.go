package server

// TODO:
// - user new muxer to verify proper HTTP methods
// - logging middleware
// - log when listening

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/lmittmann/tint"
	"golang.org/x/crypto/bcrypt"

	"github.com/llimllib/tinychat/server/db"
	"github.com/llimllib/tinychat/server/middleware"
)

func fatal(logger *slog.Logger, message string, err error, args ...any) {
	args = append(args, "error")
	args = append(args, err)
	logger.Error(message, args...)
	panic(message)
}

type ChatServer struct {
	db         *db.DB
	logger     *slog.Logger
	sessionKey string
}

func NewChatServer(level string, dbLocation string) (*ChatServer, error) {
	logger := initLog(level)
	db, err := db.NewDB(dbLocation, logger)
	if err != nil {
		return nil, err
	}
	db.RunSQLFile("schema.sql")
	return &ChatServer{
		db:         db,
		logger:     logger,
		sessionKey: "tinychat-session-key",
	}, nil
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

	if _, err := h.db.Exec("INSERT INTO users(id, username, password) VALUES(?, ?, ?)", nil, uid, user, encPass); err != nil {
		fatal(h.logger, "insert error", err)
	}
	h.logger.Debug("inserted user", "username", r.FormValue("username"))
	http.Redirect(w, r, "/", http.StatusFound)
}

// generateSessionID generates a random session ID
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (h *ChatServer) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Debug("wrong method")
		http.Redirect(w, r, "/", http.StatusFound)
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

	rows, err := h.db.Select(`
		SELECT id, password
		FROM users
		WHERE username=?`)
	if err != nil {
		fatal(h.logger, "query error", err)
	}
	if !rows.Next() {
		h.logger.Debug("missing user")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	var userID string
	var hashedPass string
	err = rows.Scan(&userID, &hashedPass)
	if err != nil {
		h.logger.Debug("failed getting user")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(pass)); err == nil {
		h.logger.Debug("login succeeded")

		// set a session cookie
		sid := generateSessionID()
		if _, err := h.db.Exec("INSERT INTO sessions(id, username, created_at) VALUES(?, ?, ?)", sid, user, time.Now()); err != nil {
			fatal(h.logger, "session insert error", err)
		}
		http.SetCookie(w, &http.Cookie{
			Name:     h.sessionKey,
			Value:    sid,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true, // Client-side scripts cannot access the cookie
		})

		http.Redirect(w, r, "/chat", http.StatusFound)
	} else {
		h.logger.Debug("wrong password")
		http.Redirect(w, r, "/", http.StatusFound)
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

// create a logger with its log level based on the LOG_LEVEL environment var,
// defaulting to INFO
func initLog(level string) *slog.Logger {
	var levelObj slog.Level
	if err := levelObj.UnmarshalText([]byte(level)); err != nil {
		fatal(slog.Default(), "Unable to convert log level", err)
	}
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level: levelObj,
	}))
	logger.Debug("started logger", "level", level)
	return logger
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

	authRequired := middleware.AuthMiddleware(h.db, h.logger, h.sessionKey)

	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))).ServeHTTP
	http.HandleFunc("/static/", h.middleware("/static", staticHandler))
	http.HandleFunc("/chat", h.middleware("/chat", authRequired(h.serveChat)))
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

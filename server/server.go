package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"golang.org/x/crypto/bcrypt"

	"github.com/llimllib/hatchat/server/api"
	"github.com/llimllib/hatchat/server/db"
	"github.com/llimllib/hatchat/server/middleware"
	"github.com/llimllib/hatchat/server/models"
	"github.com/llimllib/hatchat/server/rest"
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
	db, err := initDb(dbLocation, logger)
	if err != nil {
		return nil, err
	}

	// Seed development users if env var is set
	if os.Getenv("SEED_DEVELOPMENT_DB") != "" {
		logger.Info("SEED_DEVELOPMENT_DB is set, seeding dev users")
		if err := seedDevUsers(db, logger); err != nil {
			return nil, fmt.Errorf("seed dev users: %w", err)
		}
		if err := seedDevMessages(db, logger); err != nil {
			return nil, fmt.Errorf("seed dev messages: %w", err)
		}
	}

	return &ChatServer{
		db:         db,
		logger:     logger,
		sessionKey: "hatchat-session-key",
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
		h.logger.Debug("unable to encrypt pass", "err", err)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	uid := models.GenerateUserID()

	// Users automatically get inserted into the default room
	room, err := models.GetDefaultRoom(context.Background(), h.db)
	if err != nil {
		h.logger.Error("unable to get default room", "err", err)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	userp := &models.User{
		ID:         uid,
		Username:   user,
		Password:   string(encPass),
		LastRoom:   room.ID,
		CreatedAt:  time.Now().Format(time.RFC3339),
		ModifiedAt: time.Now().Format(time.RFC3339),
	}
	err = userp.Insert(r.Context(), h.db)
	if err != nil {
		h.logger.Debug("unable to insert user", "err", err)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	roomm := &models.RoomsMember{
		UserID: uid,
		RoomID: room.ID,
	}
	if err = roomm.Insert(r.Context(), h.db); err != nil {
		h.logger.Error("unable to add user to room", "uid", uid, "roomid", room.ID, "err", err)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	h.logger.Debug("inserted user", "username", r.FormValue("username"))
	// XXX: consider the user logged in, set a session, and redirect to chat?
	// currently this makes you go back and log in after registering
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *ChatServer) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Debug("wrong method")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	username := r.FormValue("username")
	if username == "" {
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

	user, err := models.UserByUsername(r.Context(), h.db, username)
	if err != nil {
		h.logger.Debug("Unable to find user", "user", username)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(pass)); err == nil {
		h.logger.Debug("login succeeded")

		sid := models.GenerateSessionID()
		session := models.Session{
			ID:        sid,
			UserID:    user.ID,
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		if err := session.Insert(r.Context(), h.db); err != nil {
			fatal(h.logger, "session insert error", err)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     h.sessionKey,
			Value:    sid,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true, // Client-side scripts cannot access the cookie
		})

		http.Redirect(w, r, fmt.Sprintf("/chat/%s", user.LastRoom), http.StatusFound)
	} else {
		h.logger.Debug("wrong password")
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

// create a logger with the given log level
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

func initDb(location string, logger *slog.Logger) (*db.DB, error) {
	db, err := db.NewDB(location, logger)
	if err != nil {
		return nil, err
	}

	err = db.RunSQLFile("schema.sql")
	if err != nil {
		return nil, err
	}

	// If there are no rooms, create a default room
	row := db.QueryRowContext(context.Background(), "SELECT count(*) FROM rooms")
	var n int
	err = row.Scan(&n)
	if err != nil {
		return nil, err
	}

	if n == 0 {
		room := models.Room{
			ID:        models.GenerateRoomID(),
			Name:      "main",
			RoomType:  "channel",
			IsPrivate: models.FALSE,
			IsDefault: models.TRUE,
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		if err := room.Insert(context.Background(), db); err != nil {
			return nil, err
		}
	}

	return db, nil
}

func (h *ChatServer) middleware(route string, handler http.HandlerFunc) http.HandlerFunc {
	requestID := middleware.RequestIDMiddleware(h.logger)
	logReq := middleware.RequestLogMiddleware(h.logger)(route)
	panicHandler := middleware.RecoverMiddleware(h.logger)
	return panicHandler(requestID(logReq(handler)))
}

func (h *ChatServer) Run(addr string) {
	h.logger.Info("Starting server", "addr", addr)

	hub := newHub(h.db, h.logger)
	go hub.run()

	wsAPI := api.NewApi(h.db, h.logger)
	restAPI := rest.NewAPI(h.db, h.logger)

	authRequired := middleware.AuthMiddleware(h.db, h.logger, h.sessionKey)

	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))).ServeHTTP
	http.HandleFunc("/static/", h.middleware("/static", staticHandler))
	http.HandleFunc("/chat/", h.middleware("/chat/", authRequired(h.serveChat)))
	http.HandleFunc("/search", h.middleware("/search", authRequired(h.serveChat)))
	http.HandleFunc("/register", h.middleware("/register", h.register))
	http.HandleFunc("/login", h.middleware("/login", h.login))
	http.HandleFunc("/ws", h.middleware("/ws", authRequired(func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, wsAPI, w, r)
	})))

	// REST API routes
	http.HandleFunc("/api/v1/me", h.middleware("/api/v1/me", authRequired(restAPI.MeHandler)))
	http.HandleFunc("/api/v1/me/", h.middleware("/api/v1/me/", authRequired(restAPI.MeHandler)))
	http.HandleFunc("/api/v1/rooms", h.middleware("/api/v1/rooms", authRequired(restAPI.RoomsHandler)))
	http.HandleFunc("/api/v1/rooms/", h.middleware("/api/v1/rooms/", authRequired(restAPI.RoomsHandler)))
	http.HandleFunc("/api/v1/users/", h.middleware("/api/v1/users/", authRequired(restAPI.GetUser)))

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

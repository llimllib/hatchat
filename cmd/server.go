package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/llimllib/hatchat/server"
)

var (
	addr  = flag.String("addr", "localhost:8080", "address for hatchat to listen on")
	level = flag.String("log-level", "", "log level to print logs at")
	db    = flag.String("db", "file:chat.db", "location for the chat database. Must be a url like 'file:chat.db'")
)

func main() {
	flag.Parse()

	// Use -log-level flag if provided, otherwise fall back to LOG_LEVEL env var, default to INFO
	logLevel := *level
	if logLevel == "" {
		logLevel = os.Getenv("LOG_LEVEL")
	}
	if logLevel == "" {
		logLevel = "INFO"
	}

	server, err := server.NewChatServer(logLevel, *db)
	if err != nil {
		fmt.Printf("Unable to start chat server: %v\n", err)
		os.Exit(1)
	}
	server.Run(*addr)
}

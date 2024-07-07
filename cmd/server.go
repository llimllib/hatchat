package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/llimllib/hatchat/server"
)

var (
	addr  = flag.String("addr", "localhost:8080", "address for hatchat to listen on")
	level = flag.String("log-level", "INFO", "log level to print logs at")
	db    = flag.String("db", "file:chat.db", "location for the chat database. Must be a url like 'file:chat.db'")
)

func main() {
	flag.Parse()
	server, err := server.NewChatServer(*level, *db)
	if err != nil {
		fmt.Printf("Unable to start chat server: %v\n", err)
		os.Exit(1)
	}
	server.Run(*addr)
}

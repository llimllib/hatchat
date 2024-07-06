package main

import (
	"flag"

	"github.com/llimllib/tinychat/server"
)

var (
	addr  = flag.String("addr", "localhost:8080", "address for tinychat to listen on")
	level = flag.String("log-level", "INFO", "log level to print logs at")
)

func main() {
	flag.Parse()
	server := server.NewChatServer(*level)
	server.Run(*addr)
}

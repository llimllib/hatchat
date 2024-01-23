package main

import (
	"flag"

	"github.com/llimllib/tinychat/server"
)

var addr = flag.String("addr", "localhost:8080", "tinychat address")

func main() {
	flag.Parse()
	server := server.NewChatServer()
	server.Run(*addr)
}

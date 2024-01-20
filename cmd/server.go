package main

import (
	"flag"

	"github.com/llimllib/tinychat/server"
)

var addr = flag.String("addr", ":8080", "http service address")

func main() {
	flag.Parse()
	server := server.NewChatServer()
	server.Run(*addr)
}

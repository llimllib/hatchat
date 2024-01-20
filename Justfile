build:
    go build -o tinychat github.com/llimllib/tinychat/cmd

run: build
    ./tinychat

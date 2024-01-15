build:
    go build -o tinychat github.com/llimllib/tinychat/bin

run: build
    ./tinychat

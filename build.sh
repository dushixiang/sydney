CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o sydney-linux main.go || exit
echo "success"
upx sydney-linux
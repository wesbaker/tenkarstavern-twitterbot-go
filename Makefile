build:
	go get github.com/aws/aws-lambda-go/lambda
	env GOOS=linux go build -ldflags="-s -w" -o bin/tweet tweet/main.go

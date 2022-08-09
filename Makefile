build:
	go build -ldflags="-s -w" -o carrier cmd/carrier.go

install:
	go install -ldflags="-s -w" cmd/carrier.go
.PHONY: build clean stop start restart test

build:
	go build -o ./srv-bin ./cmd/srv

clean:
	rm -f srv

test:
	go test ./...

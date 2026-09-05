.PHONY: proto test build bot

proto:
	chmod +x scripts/gen_proto.sh && ./scripts/gen_proto.sh

test:
	go test ./...

build:
	go build -o bin/server ./cmd/server
	go build -o bin/bot ./cmd/bot

bot: build
	./bin/server configs/server.yaml

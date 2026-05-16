.PHONY: test run terminal compose-up compose-terminal compose-down proto

test:
	go test ./...

run:
	go run -buildvcs=false ./cmd/api-gateway

terminal:
	go run -buildvcs=false ./cmd/terminal

compose-up:
	podman-compose up --build

compose-terminal:
	podman-compose run --rm terminal

compose-down:
	podman-compose down -v

proto:
	protoc --go_out=. --go_opt=module=tp-final-sdp --go-grpc_out=. --go-grpc_opt=module=tp-final-sdp proto/crypto_jobs.proto

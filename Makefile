.PHONY: run build requirements

run: requirements
	go run main.go

build: requirements
	CGO_ENABLED=0 go build -a -ldflags "-w -s" -o currency_info main.go
	./currency_info

requirements:
	@go mod download
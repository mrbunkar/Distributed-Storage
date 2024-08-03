EXECUTABLE := main

build:
	@go build -o $(EXECUTABLE) .

run: build
	@./$(EXECUTABLE)

test:
	@go test -v ./...
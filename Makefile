.PHONY: build demo test

build:
	go build -o skill-eval .

demo:
	vhs demo.tape

test:
	go test ./...

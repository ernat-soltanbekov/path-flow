.PHONY: build test audit demo clean
build:
	go build -o bin/path-flow .
	go build -o bin/visualizer ./cmd/visualizer

test:
	go test ./...

audit: test
	go vet ./...
	go test -race ./...

# Open path-flow.html after this target completes.
demo: build
	./bin/path-flow examples/ghost-of-astana.txt > bin/demo.txt
	./bin/visualizer -out path-flow.html < bin/demo.txt

clean:
	rm -f bin/path-flow bin/visualizer bin/demo.txt

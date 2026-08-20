.PHONY: build install modernize

build:
	go build -o ec2

install: build
	cp ec2 ~/.local/bin

modernize:
	go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest ./...

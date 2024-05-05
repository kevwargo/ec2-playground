.PHONY: build
build:
	go build -o ec2

.PHONY: install
install: build
	cp ec2 ~/.local/bin

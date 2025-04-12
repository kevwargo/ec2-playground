package main

import (
	"log"

	"kevwargo/ec2-playground/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

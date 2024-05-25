package vmformat

import (
	"log"
	"os"
)

var printer = log.New(os.Stdout, "", 0)

func Print(vm VM) {
	printer.Print(vm)
}

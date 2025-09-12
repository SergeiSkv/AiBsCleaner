package main

import (
	"log"

	"github.com/SergeiSkv/AiBsCleaner/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

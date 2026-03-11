package main

import (
	"flag"
	"fmt"
	"os"

	"bigtoy/backend/services"
)

func main() {
	var password string
	flag.StringVar(&password, "password", "", "plaintext password to hash")
	flag.Parse()

	if password == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/hashpass -password \"your-strong-password\"")
		os.Exit(1)
	}

	hash, err := services.GeneratePasswordHash(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}

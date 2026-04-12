package main

import (
	"fmt"
	"os"

	"github.com/Kpeewu/tissi-mah/services/support-service/internal/password"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: hashpwd <password>")
		os.Exit(1)
	}
	h, err := password.Hash(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(h)
}

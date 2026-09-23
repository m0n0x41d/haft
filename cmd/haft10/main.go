// Command haft10 is the isolated v10 candidate entrypoint.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Println("haft10 development (block 1, unqualified)")
		return
	}
	fmt.Fprintln(os.Stderr, "haft10: implementation in progress; only version is available")
	os.Exit(2)
}

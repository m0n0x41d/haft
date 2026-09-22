package main

import (
	"fmt"
	"github.com/m0n0x41d/haft/internal/cli"
	"os"
)

func main() {
	result, err := cli.HostRepairProbe(os.Args[1], os.Args[2], os.Args[3], os.Args[4])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(result))
}

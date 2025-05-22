package main

import (
	"clover/vm"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	debug := flag.Bool("debug", false, "Enable debug mode")
	libs := flag.String("libs", "", "Comma-separated list of libraries to load")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Println("Usage: clover [options] <file.cvm>")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	filePath := flag.Arg(0)
	if filepath.Ext(filePath) != ".cvm" {
		fmt.Printf("Error: File must have .cvm extension: %s\n", filePath)
		os.Exit(1)
	}

	code, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	vm := vm.NewVM()
	vm.EnableDebug(*debug)

	if *libs != "" {
		for _, lib := range strings.Split(*libs, ",") {
			if err := vm.LoadLib(lib); err != nil {
				fmt.Printf("Error loading library %s: %v\n", lib, err)
				os.Exit(1)
			}
		}
	}

	instructions, err := vm.Parse(string(code))
	if err != nil {
		fmt.Printf("Error parsing bytecode: %v\n", err)
		os.Exit(1)
	}

	if err := vm.Execute(instructions); err != nil {
		fmt.Printf("Error executing bytecode: %v\n", err)
		os.Exit(1)
	}
}

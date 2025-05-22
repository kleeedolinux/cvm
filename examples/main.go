package main

import (
	"clover/examples/concurrency"
	"clover/examples/fibonacci"
	"fmt"
)

func main() {
	fmt.Println("Running Fibonacci example:")
	fibonacci.Run()

	fmt.Println("\nRunning Concurrency example:")
	concurrency.Run()
}

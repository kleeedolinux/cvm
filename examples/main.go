package main

import (
	"clover/examples/advanced"
	"clover/examples/concurrency"
	"clover/examples/fibonacci"
	"fmt"
)

func main() {
	fmt.Println("Running Fibonacci example:")
	fibonacci.Run()
	fmt.Println()

	fmt.Println("Running Concurrency example:")
	concurrency.Run()
	fmt.Println()

	fmt.Println("Running Advanced example:")
	advanced.Run()
	fmt.Println()
}

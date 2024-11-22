package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

func main() {

	// Initialize the counter variable
	var counter int = 1

	go func() {
		for {
			// Write the current counter value to stdout
			fmt.Printf("%d\n", counter)

			// Increment the counter by 1
			counter++

			// Wait for 1 second before writing again
			time.Sleep(1 * time.Second)
		}
	}()
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		fmt.Fprintf(os.Stderr, "srv: recv %s\n", s.Text())
	}
	err := s.Err()
	if err != nil {
		panic(err)
	}
}

package main

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"time"
)

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	cmd := exec.Command("./main")

	cmdIn, err := cmd.StdinPipe()
	checkErr(err)
	cmdOut, err := cmd.StdoutPipe()
	checkErr(err)

	err = cmd.Start()
	checkErr(err)
	defer cmd.Wait()

	// Initialize the counter variable
	var counter int = 0

	go func() {
		for {
			// Write the current counter value to stdout
			fmt.Fprintf(cmdIn, "%d\n", counter)
			fmt.Printf("client: send %d\n", counter)

			// Increment the counter by 1
			counter++

			// Wait for 1 second before writing again
			time.Sleep(1 * time.Second)
		}
	}()
	s := bufio.NewScanner(cmdOut)
	for s.Scan() {
		fmt.Printf("client: recv %s\n", s.Text())
	}
	checkErr(s.Err())
}

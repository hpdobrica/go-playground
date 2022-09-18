package main

import (
	"fmt"
	"time"
)

func step(arg int) {
	fmt.Println(arg)

	time.Sleep(1 * time.Second)

}

func main() {
	args := []int{1, 2, 1, 3}

	for i := 0; i >= 0; i++ {
		step(args[i%4])
	}
}

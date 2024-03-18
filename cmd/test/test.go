package main

import "fmt"

type data struct {
	Type string `json:"type"`
}

func main() {
	var a []string
	fmt.Println(cap(a))
}

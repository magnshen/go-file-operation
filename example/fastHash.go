package main

import (
	"fmt"
	"github.com/magnshen/go-file-operation/fastHash"
)

func main() {
	str, err := fastHash.Sum("./example/go1.15.5.darwin-amd64.pkg")
	if err != nil {
		fmt.Println("Hash sum error")
	} else {
		fmt.Printf("hash = %s\n", str)
	}
}

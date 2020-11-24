package main

import (
	"fmt"
	"github.com/magnshen/go-file-operation/download"
)

func main() {
	download := download.Default()
	download.
		Get("https://studygolang.com/dl/golang/go1.15.5.darwin-amd64.pkg").
		Append("./example/go1.15.5.darwin-amd64.pkg").
		SetProgressCallBackHandle(func(finish, total int64) bool {
			fmt.Printf("total : %d ;finish : %d \n", total, finish)
			//if finish >= 1<<12{
			//	return false   //暂停
			//}
			return true
		}).SetCompleteCallBackHandle(func(err error) {
		if err != nil {
			fmt.Printf("download error: %v \n", err)
		} else {
			fmt.Printf("download success:\n")
		}
	})
	go download.Start()
	download.Wait()
}

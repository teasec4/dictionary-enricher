package main

import (
	"dabkrs-examples/internal/parser"
	"fmt"
)

func main(){
	const filePath = "zh_cn_50k.txt"
	const limitLines = 20
	err := parser.OpenFile(filePath, limitLines)
	if err != nil{
		fmt.Printf("error: %v\n", err)
	}
}
package parser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)


func OpenFile(path string, limit int) (error){
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл: %w", err)
	}
	defer file.Close()

	
	reader := bufio.NewReader(file)
	
	var stopLine = limit
	counter := 0
	for{
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			// логируем ошибку и продолжаем
			return fmt.Errorf("ошибка чтения строки: %v\n", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			if err == io.EOF {
				break
			}
			continue
		}
		
		counter++
		parts := strings.SplitN(line, " ", 2)
		word := parts[0]
		fmt.Println(word)
		
		if counter >= stopLine{
			break
		}
		// for _, l := range line{
		// 	var headword []rune
		// 	if isCJK(l){
		// 		headword = append(headword, l)
		// 	}
		// }
	}

	return nil
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
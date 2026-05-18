package filesystem

import (
	"bufio"
	"log"
	"os"
	"regexp"
)

func GetTxtUrls(txtPath string) ([]string, error) {
	file, err := os.Open(txtPath)
	if err != nil {
		log.Println("ERROR OPENING FILE: ", err.Error())
		return nil, err
	}

	defer file.Close()

	var urls []string

	re, err := regexp.Compile(`https?://[^\s/$.?#].[^\s]*`)
	if err != nil {
		log.Println("ERROR COMPILING REGEX: ", err.Error())
		return nil, err
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindAllString(line, -1)

		if len(matches) > 0 {
			urls = append(urls, matches...)
		}
	}

	err = scanner.Err()

	if err != nil {
		log.Println("Failed to Read file: ", err.Error())
		return nil, err
	}

	log.Printf("FOUND %v URLs", len(urls))
	return urls, nil
}

package utils

import "strings"

func GetLastItemAfterSplit(str, separator string) string {
	split := strings.Split(str, separator)
	return split[len(split)-1]
}

func JoinImageNames(images []string) string {
	response := ""
	for _, image := range images {
		response += "- " + image + "\n"
	}
	return response
}

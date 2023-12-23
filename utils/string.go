package utils

import "strings"

func GetLastItemAfterSplit(str, separator string) string {
	split := strings.Split(str, separator)
	return split[len(split)-1]
}

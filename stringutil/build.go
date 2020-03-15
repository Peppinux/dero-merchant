package stringutil

import "strings"

// Build returns a new string made up of multiple strings
func Build(strs ...string) string {
	var sb strings.Builder
	for _, str := range strs {
		sb.WriteString(str)
	}
	return sb.String()
}

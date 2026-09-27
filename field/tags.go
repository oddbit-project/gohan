package field

import (
	"reflect"
	"strings"
)

// parseTagList attempt to parse tag from a list of possible tags
// if a match from the list of possible tags is found, returns the list of options
func parseTagList(field reflect.StructField, tags []string) []string {
	result := make([]string, 0)
	found := false
	for _, tag := range tags {
		if tv := field.Tag.Get(tag); len(tv) > 0 {
			result = append(result, strings.Split(tv, ",")...)
			found = true
		}
		if found {
			break
		}
	}
	return result
}

// parseTag attempt to parse tag
func parseTag(field reflect.StructField, tag string) []string {
	if tv := field.Tag.Get(tag); len(tv) > 0 {
		return strings.Split(tv, ",")
	}
	return []string{}
}

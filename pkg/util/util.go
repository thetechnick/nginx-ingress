package util

import (
	"strconv"
	"strings"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// There seems to be no composite interface in the kubernetes api package,
// so we have to declare our own.
type apiObject interface {
	meta_v1.Object
	runtime.Object
}

// GetMapKeyAsBool searches the map for the given key and parses the key as bool
func GetMapKeyAsBool(m map[string]string, key string) (bool, bool, error) {
	if str, exists := m[key]; exists {
		b, err := strconv.ParseBool(str)
		if err != nil {
			return false, exists, err
		}
		return b, exists, nil
	}
	return false, false, nil
}

// GetMapKeyAsInt tries to find and parse a key in a map as int64
func GetMapKeyAsInt(m map[string]string, key string) (int64, bool, error) {
	if str, exists := m[key]; exists {
		i, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return 0, exists, err
		}
		return i, exists, nil
	}
	return 0, false, nil
}

// GetMapKeyAsStringSlice tries to find and parse a key in the map as string slice splitting it on delimiter
func GetMapKeyAsStringSlice(m map[string]string, key string, context apiObject, delimiter string) ([]string, bool) {
	if str, exists := m[key]; exists {
		slice := strings.Split(str, delimiter)
		return slice, exists
	}
	return nil, false
}

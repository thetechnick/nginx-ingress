package parser

import (
	"fmt"
	"strings"
)

// ValidationError packs multiple validation errors into a single error
type ValidationError []error

func (e ValidationError) Error() string {
	if e == nil {
		return fmt.Sprintf("nil error!")
	}
	errString := []string{}
	for _, err := range e {
		errString = append(errString, err.Error())
	}
	return fmt.Sprintf("validation: [ %s ]", strings.Join(errString, ", "))
}

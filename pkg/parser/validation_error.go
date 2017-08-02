package parser

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
)

// ValidationError contains individual errors during validation
type ValidationError struct {
	Object runtime.Object
	Errors []error
}

func (e *ValidationError) Error() string {
	if e == nil {
		return fmt.Sprintf("nil error!")
	}
	return fmt.Sprintf("error validating: %v", e.Errors)
}

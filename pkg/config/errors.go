package config

import (
	"fmt"
	"strings"
)

// IngressAnnotationError is a config error for annotation of the Ingress object
type IngressAnnotationError struct {
	Annotation      string
	ValidationError error
}

func (e *IngressAnnotationError) Error() string {
	return fmt.Sprintf("Skipping annotation %q: %v", e.Annotation, e.ValidationError)
}

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

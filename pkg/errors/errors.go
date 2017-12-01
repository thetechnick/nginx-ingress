package errors

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/pkg/api/v1"
)

// ErrObjectContext describes errors with an attached kubernetes object
type ErrObjectContext interface {
	Object() runtime.Object
	WrappedError() error
	Error() string
}

// WrapInObjectContext wraps an error into a new ErrObjectContext
func WrapInObjectContext(err error, obj runtime.Object) ErrObjectContext {
	return &errObjectContext{
		err:    err,
		object: obj,
	}
}

type errObjectContext struct {
	err    error
	object runtime.Object
}

func (e *errObjectContext) Object() runtime.Object {
	return e.object
}

func (e *errObjectContext) WrappedError() error {
	return e.err
}

func (e *errObjectContext) Error() string {
	ref, err := v1.GetReference(scheme.Scheme, e.object)
	if err != nil {
		glog.Errorf(
			"Could not construct reference to: '%#v' due to: '%v'. Will not show context of error",
			e.object,
			err,
		)
		return e.err.Error()
	}

	return fmt.Sprintf(
		"[Context: %s (%s/%s)]: %s",
		ref.Kind,
		ref.Name,
		ref.Namespace,
		e.err.Error(),
	)
}

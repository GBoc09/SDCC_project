package reporting

import (
	"errors"
	"testing"
)

func TestErrorHandlerReportsConfiguredError(t *testing.T) {
	handler := &ErrorHandler{}
	want := errors.New("background failure")

	var got error
	handler.SetErrorHandler(func(err error) {
		got = err
	})

	handler.Report(want)

	if !errors.Is(got, want) {
		t.Fatalf("reported error = %v, want %v", got, want)
	}
}

func TestErrorHandlerWithoutCallbackIsSafe(t *testing.T) {
	handler := &ErrorHandler{}
	handler.Report(errors.New("ignored failure"))
}

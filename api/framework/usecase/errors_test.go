package usecase_test

import (
	"errors"
	"testing"

	fwusecase "github.com/tfnick/go-svelte-starter/api/framework/usecase"
)

func TestErrorCodeAndMessageHelpers(t *testing.T) {
	cause := errors.New("database detail")
	err := fwusecase.E(fwusecase.CodeNotFound, "user not found", cause)

	if fwusecase.CodeOf(err) != fwusecase.CodeNotFound {
		t.Fatalf("expected not_found code, got %q", fwusecase.CodeOf(err))
	}
	if fwusecase.MessageOf(err, "fallback") != "user not found" {
		t.Fatalf("expected usecase message, got %q", fwusecase.MessageOf(err, "fallback"))
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause")
	}
}

func TestPlainErrorsAreInternal(t *testing.T) {
	err := errors.New("raw storage detail")

	if fwusecase.CodeOf(err) != fwusecase.CodeInternal {
		t.Fatalf("expected plain error to be internal, got %q", fwusecase.CodeOf(err))
	}
	if fwusecase.MessageOf(err, "safe message") != "safe message" {
		t.Fatalf("expected fallback safe message")
	}
}

func TestLogErrorOfReturnsUnderlyingCause(t *testing.T) {
	cause := errors.New("database detail")
	err := fwusecase.E(fwusecase.CodeInternal, "safe message", cause)

	if fwusecase.LogErrorOf(err) != cause {
		t.Fatalf("expected log error to expose wrapped cause")
	}
}

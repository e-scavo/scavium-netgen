package app

import (
	"context"
	"testing"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
)

func TestCloseCancelsRuntimeContext(t *testing.T) {
	application := New(config.Defaults())

	if err := application.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case <-application.ctx.Done():
	default:
		t.Fatal("runtime context was not cancelled")
	}
}

func TestCloseRunsRegisteredClosers(t *testing.T) {
	application := New(config.Defaults())
	called := false
	application.closeFuncs = append(application.closeFuncs, func(context.Context) error {
		called = true
		return nil
	})

	if err := application.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !called {
		t.Fatal("registered closer was not called")
	}
}

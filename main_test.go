package main

import (
	"testing"
)

func TestBuildApp(t *testing.T) {
	t.Parallel()
	app := buildApp()

	if err := app.Validate(); err != nil {
		t.Fatal(err)
	}
}

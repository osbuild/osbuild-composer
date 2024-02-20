package builder_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/osbuild/osbuild-composer/pkg/jobsite/builder"
)

func TestBuilderState(t *testing.T) {
	t.Run("SetState", func(t *testing.T) {
		b := builder.Builder{
			State:        builder.StateClaim,
			StateChannel: make(chan builder.State, 16),
		}

		b.SetState(builder.StateDone)

		if b.GetState() != builder.StateDone {
			t.Errorf("Failed to SetState")
		}

		s := <-b.StateChannel

		if s != builder.StateDone {
			t.Errorf("Incorrect state in StateChannel")
		}

		// try to go back in state
		b.SetState(builder.StateClaim)

		if b.GetState() != builder.StateError {
			t.Errorf("State regression did not result in StateError")
		}

		s = <-b.StateChannel

		if s != builder.StateError {
			t.Errorf("Incorrect state in StateChannel")
		}

	})

	t.Run("GetState", func(t *testing.T) {
		b := builder.Builder{
			State:        builder.StateClaim,
			StateChannel: make(chan builder.State, 16),
		}

		if b.GetState() != builder.StateClaim {
			t.Errorf("Failed to GetState")
		}

		b.SetState(builder.StateBuild)

		if b.GetState() != builder.StateBuild {
			t.Errorf("Failed to GetState")
		}

		s := <-b.StateChannel

		if s != builder.StateBuild {
			t.Errorf("Incorrect state in StateChannel")
		}

	})

	t.Run("GuardState", func(t *testing.T) {
		b := builder.Builder{
			State:        builder.StateClaim,
			StateChannel: make(chan builder.State, 16),
		}

		err := b.GuardState(builder.StateClaim)

		if err != nil {
			t.Errorf("Failed GuardState")
		}

		b.SetState(builder.StateBuild)

		err = b.GuardState(builder.StateClaim)

		if err == nil {
			t.Errorf("Failed GuardState")
		}
	})
}

func TestHandleClaim(t *testing.T) {
	t.Run("Happy", func(t *testing.T) {
		b := builder.Builder{
			State:        builder.StateClaim,
			StateChannel: make(chan builder.State, 16),
		}
		r := httptest.NewRecorder()

		req, err := http.NewRequest("POST", "/claim", http.NoBody)

		if err != nil {
			t.Fatal(err)
		}

		b.Mux().ServeHTTP(r, req)

		if r.Result().StatusCode != http.StatusOK {
			t.Errorf("Invalid status code")
		}

		if b.GetState() != builder.StateProvision {
			t.Errorf("Did not progress state after request")
		}
	})
}

func TestHandleProvision(t *testing.T) {
	t.Run("Happy", func(t *testing.T) {
		d := t.TempDir()
		b := builder.Builder{
			State:        builder.StateProvision,
			StateChannel: make(chan builder.State, 16),
			BuildPath:    d,
		}
		r := httptest.NewRecorder()

		req, err := http.NewRequest("PUT", "/provision", http.NoBody)

		if err != nil {
			t.Fatal(err)
		}

		b.Mux().ServeHTTP(r, req)

		if r.Result().StatusCode != http.StatusCreated {
			t.Errorf("Invalid status code")
		}

		if b.GetState() != builder.StatePopulate {
			t.Errorf("Did not progress state after request")
		}

		if _, err := os.Stat(filepath.Join(d, "manifest.json")); err != nil {
			t.Fatal(err)
		}
	})
}

func TestHandlePopulate(t *testing.T) {
	t.Run("Happy", func(t *testing.T) {
		b := builder.Builder{
			State:        builder.StatePopulate,
			StateChannel: make(chan builder.State, 16),
		}
		r := httptest.NewRecorder()

		req, err := http.NewRequest("POST", "/populate", http.NoBody)

		if err != nil {
			t.Fatal(err)
		}

		b.Mux().ServeHTTP(r, req)

		if r.Result().StatusCode != http.StatusOK {
			t.Errorf("Invalid status code")
		}

		if b.GetState() != builder.StateBuild {
			t.Errorf("Did not progress state after request")
		}
	})
}

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// vehiclesTestPlate returns a plate that cannot collide with another test run.
// The name is slice-specific because appointments_test.go owns uniquePlate.
func vehiclesTestPlate(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestCreateVehicleReturns201(t *testing.T) {
	env, pool := newCustomersTestEnv(t)

	plate := vehiclesTestPlate("B-TEST")
	rec := customersPostJSON(t, env.handler, "/api/vehicles", vehicleCreateRequest{
		Plate:   plate,
		Make:    "Volkswagen",
		Model:   "Golf",
		Mileage: 123456,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/vehicles: want 201, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var created vehicleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("create vehicle body is not JSON: %v", err)
	}
	if created.ID <= 0 {
		t.Fatalf("created vehicle has no id: %+v", created)
	}
	if created.Plate != plate || created.Make != "Volkswagen" || created.Model != "Golf" || created.Mileage != 123456 {
		t.Fatalf("created vehicle mismatch: %+v", created)
	}

	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM vehicles WHERE id = $1`, created.ID); err != nil {
			t.Errorf("cleanup vehicle: %v", err)
		}
	})
}

// TestDuplicatePlateReturns409 is the behaviour the sprint checks end to end: a
// second vehicle with the same plate is rejected with 409 (AC-02).
func TestDuplicatePlateReturns409(t *testing.T) {
	env, pool := newCustomersTestEnv(t)

	plate := vehiclesTestPlate("B-DUP")
	body := vehicleCreateRequest{Plate: plate, Make: "Audi", Model: "A4", Mileage: 1000}

	first := customersPostJSON(t, env.handler, "/api/vehicles", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first POST /api/vehicles: want 201, got %d (body=%q)", first.Code, first.Body.String())
	}
	var created vehicleResponse
	if err := json.Unmarshal(first.Body.Bytes(), &created); err != nil {
		t.Fatalf("first vehicle body is not JSON: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM vehicles WHERE id = $1`, created.ID); err != nil {
			t.Errorf("cleanup vehicle: %v", err)
		}
	})

	second := customersPostJSON(t, env.handler, "/api/vehicles", body)
	if second.Code != http.StatusConflict {
		t.Fatalf("duplicate plate: want 409, got %d (body=%q)", second.Code, second.Body.String())
	}
	decodeError(t, second.Body.Bytes())
}

func TestCreateVehicleInvalidValuesReturn400(t *testing.T) {
	env, _ := newCustomersTestEnv(t)

	cases := []struct {
		name string
		body vehicleCreateRequest
	}{
		{"missing plate", vehicleCreateRequest{Plate: "   ", Make: "VW", Model: "Golf", Mileage: 10}},
		{"negative mileage", vehicleCreateRequest{Plate: vehiclesTestPlate("B-NEG"), Make: "VW", Model: "Golf", Mileage: -1}},
	}
	for _, tc := range cases {
		rec := customersPostJSON(t, env.handler, "/api/vehicles", tc.body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: want 400, got %d (body=%q)", tc.name, rec.Code, rec.Body.String())
		}
		decodeError(t, rec.Body.Bytes())
	}
}

// TestVehiclePlateIsNormalisedToUpper documents that the stored plate is
// uppercased, so two spellings of the same plate collide.
func TestVehiclePlateIsNormalisedToUpper(t *testing.T) {
	env, pool := newCustomersTestEnv(t)

	lower := vehiclesTestPlate("b-mix")
	upper := strings.ToUpper(lower)

	first := customersPostJSON(t, env.handler, "/api/vehicles", vehicleCreateRequest{
		Plate: lower, Make: "VW", Model: "Polo", Mileage: 5,
	})
	if first.Code != http.StatusCreated {
		t.Fatalf("first POST: want 201, got %d (body=%q)", first.Code, first.Body.String())
	}
	var created vehicleResponse
	if err := json.Unmarshal(first.Body.Bytes(), &created); err != nil {
		t.Fatalf("vehicle body is not JSON: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM vehicles WHERE id = $1`, created.ID); err != nil {
			t.Errorf("cleanup vehicle: %v", err)
		}
	})
	if created.Plate != upper {
		t.Fatalf("stored plate: want %q, got %q", upper, created.Plate)
	}

	second := customersPostJSON(t, env.handler, "/api/vehicles", vehicleCreateRequest{
		Plate: upper, Make: "VW", Model: "Polo", Mileage: 5,
	})
	if second.Code != http.StatusConflict {
		t.Fatalf("same plate in another case: want 409, got %d (body=%q)", second.Code, second.Body.String())
	}
}

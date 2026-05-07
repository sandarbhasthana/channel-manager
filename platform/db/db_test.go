package db

import "testing"

func TestConfigDSN(t *testing.T) {
	c := Config{
		Host: "localhost", Port: 5432, DBName: "channel",
		User: "postgres", Password: "secret", SSLMode: "disable",
	}
	got := c.DSN()
	want := "host=localhost port=5432 dbname=channel user=postgres password=secret sslmode=disable"
	if got != want {
		t.Fatalf("DSN mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestConfigURL(t *testing.T) {
	c := Config{
		Host: "localhost", Port: 5432, DBName: "channel",
		User: "postgres", Password: "secret", SSLMode: "disable",
	}
	got := c.URL()
	want := "postgres://postgres:secret@localhost:5432/channel?sslmode=disable"
	if got != want {
		t.Fatalf("URL mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestSchemasOrder(t *testing.T) {
	// Tenancy must be first; reservations must be last (depends on
	// pms, pricing, channel ids logically).
	if Schemas[0] != "tenancy" {
		t.Fatalf("expected tenancy first, got %q", Schemas[0])
	}
	if Schemas[len(Schemas)-1] != "reservations" {
		t.Fatalf("expected reservations last, got %q", Schemas[len(Schemas)-1])
	}
}

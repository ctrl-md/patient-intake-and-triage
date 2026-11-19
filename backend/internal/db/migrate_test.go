package db

import (
	"reflect"
	"testing"
)

func TestPending(t *testing.T) {
	names := []string{"0001_init.sql", "0002_more.sql", "0003_later.sql"}
	applied := map[string]bool{"0001_init.sql": true}

	got := pending(names, applied)
	want := []string{"0002_more.sql", "0003_later.sql"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMigrationNamesAreSorted(t *testing.T) {
	names, err := migrationNames(migrationFiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no migrations found")
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("not sorted: %v", names)
		}
	}
}

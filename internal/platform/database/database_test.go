package database

import "testing"

func TestRebind(t *testing.T) {
	pg := &DB{Provider: "postgres"}
	got := pg.Rebind("SELECT * FROM users WHERE id=? AND organization_id=?")
	want := "SELECT * FROM users WHERE id=$1 AND organization_id=$2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	mysql := &DB{Provider: "mysql"}
	q := "SELECT * FROM users WHERE id=?"
	if got := mysql.Rebind(q); got != q {
		t.Fatalf("mysql query changed: %q", got)
	}
}

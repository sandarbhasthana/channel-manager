//go:build ignore

package main

import (
	"context"
	"fmt"


	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:Pulak91@localhost:5432/channel?sslmode=disable")
	if err != nil {
		panic(err)
	}

	var email, fullName string
	var preferences []byte
	err = pool.QueryRow(context.Background(), `
		SELECT u.email, u.full_name, m.preferences
		FROM tenancy.users u
		LEFT JOIN tenancy.memberships m ON u.id = m.user_id AND m.org_id = $2
		WHERE u.id = $1
	`, "user_01KRM27YNRA2WG9TPKR6G516KT", "03436af4-b0c1-4c65-9fcc-4bb289404684").Scan(&email, &fullName, &preferences)
	
	if err != nil {
		fmt.Println("GET ERROR:", err)
	} else {
		fmt.Println("GET SUCCESS:", string(preferences))
	}

	_, err = pool.Exec(context.Background(), `
		INSERT INTO tenancy.memberships (org_id, user_id, preferences)
		VALUES ($3, $2, $1)
		ON CONFLICT (org_id, user_id)
		DO UPDATE SET preferences = $1, updated_at = now()
	`, []byte(`{"default_property_id":"123"}`), "user_01KRM27YNRA2WG9TPKR6G516KT", "03436af4-b0c1-4c65-9fcc-4bb289404684")

	if err != nil {
		fmt.Println("UPDATE ERROR:", err)
	} else {
		fmt.Println("UPDATE SUCCESS")
	}
}

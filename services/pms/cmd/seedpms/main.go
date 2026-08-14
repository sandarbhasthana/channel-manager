package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/channel-manager/channel-manager/platform/auth"
	pmsports "github.com/channel-manager/channel-manager/services/pms/ports"
	pmsusecases "github.com/channel-manager/channel-manager/services/pms/usecases"
	pmspostgres "github.com/channel-manager/channel-manager/services/pms/adapters/postgres"
)

func main() {
	dbURL := "postgres://postgres:Pulak91@localhost:5432/channel?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	connRepo := pmspostgres.NewConnectionRepository(pool)
	propRepo := pmspostgres.NewPropertyRepository(pool)
	rtRepo := pmspostgres.NewRoomTypeRepository(pool)
	roomRepo := pmspostgres.NewRoomRepository(pool)
	secretRepo := pmspostgres.NewSecretRepository(pool, "test-key-32-bytes-long-for-aes256")
	
	svc := pmsusecases.NewPmsService(connRepo, propRepo, rtRepo, roomRepo, secretRepo, nil)

	// create context with OrgID
	ctx = auth.WithTenantContext(ctx, auth.TenantContext{
		OrgID: "a3fd9792-e927-45c5-9e05-d10168a0dadf",
		Role:  "admin",
	})

	creds := map[string]string{
		"base_url": "http://localhost:3001",
		"token":    "test-token",
	}

	conn, err := svc.ConnectPms(ctx, "mypms", "Local PMS App", creds)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Connected PMS successfully! ID: %s\n", conn.ID)
}

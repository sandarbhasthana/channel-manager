package main

import (
	"context"
	"fmt"
	"os"
	"encoding/json"

	platformdb "github.com/channel-manager/channel-manager/platform/db"
	"github.com/channel-manager/channel-manager/services/pms/adapters/postgres"
	"github.com/channel-manager/channel-manager/services/pms/usecases"
)

type dummyLogger struct{}
func (dummyLogger) Info(msg string, args ...any) {}
func (dummyLogger) Error(msg string, args ...any) {}
func (dummyLogger) Warn(msg string, args ...any) {}
func (dummyLogger) Debug(msg string, args ...any) {}
func (dummyLogger) With(args ...any) usecases.Logger { return dummyLogger{} }

func main() {
	ctx := context.Background()
	pool, err := platformdb.NewPool(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Println("Error connecting to db", err)
		return
	}
	
	log := dummyLogger{}
	propRepo := postgres.NewPropertyRepository(pool)
	rtRepo := postgres.NewRoomTypeRepository(pool)
	roomRepo := postgres.NewRoomRepository(pool)
	
	s := usecases.NewPmsService(nil, propRepo, rtRepo, roomRepo, nil, log)
	
	rts, err := s.ListRoomTypes(ctx, "996ecdcf-e18b-4c1e-af92-ebc7ac65b332")
	if err != nil {
		fmt.Println("ListRoomTypes error:", err)
		return
	}
	
	b, _ := json.MarshalIndent(rts, "", "  ")
	fmt.Println(string(b))
}

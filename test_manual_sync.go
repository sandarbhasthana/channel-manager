package main

import (
	"context"
	"fmt"
	"encoding/json"

	"github.com/channel-manager/channel-manager/services/pms/adapters/mypms"
)

func main() {
	client := mypms.NewClient(mypms.Config{
		BaseURL: "http://localhost:4001",
		Token: "dev-pms-integration-token",
	})
	resp, err := client.GetRoomDetails(context.Background(), "cmpkvc8fe00029kbmmgkx0pyc", "", "")
	if err != nil {
		fmt.Println("GetRoomDetails failed", "err", err)
		return
	}
	
	for _, rt := range resp.RoomTypesList() {
		fmt.Printf("Room Type: %s (ExtID: %s)\n", rt.Name, rt.RoomTypeID)
		b, _ := json.Marshal(rt)
		fmt.Println(string(b))
		for _, rm := range rt.Rooms {
			fmt.Printf("  -> Room: %s (ExtID: %s)\n", rm.Name, rm.RoomID)
		}
	}
}

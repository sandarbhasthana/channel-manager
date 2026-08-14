package main

import (
	"context"
	"fmt"
	"github.com/channel-manager/channel-manager/services/pms/adapters/mypms"
)

func main() {
	client := mypms.NewClient(mypms.Config{BaseURL: "http://localhost:3000", Token: "mock"})
	resp, err := client.GetRoomDetails(context.Background(), "cmpkvc8fe00029kbmmgkx0pyc", "", "")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Resp: %+v\n", resp)
	fmt.Printf("RoomTypesList: %+v\n", resp.RoomTypesList())
	fmt.Printf("RoomsList: %+v\n", resp.RoomsList())
	for _, rt := range resp.RoomTypesList() {
		fmt.Printf("RT %s Rooms: %+v\n", rt.ID, rt.Rooms)
	}
}

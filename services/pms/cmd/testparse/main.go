package main

import (
	"encoding/json"
	"fmt"
	"github.com/channel-manager/channel-manager/services/pms/adapters/mypms"
)

func main() {
	jsonData := []byte(`{
		"data": {
			"room_types": [
				{
					"id": "rt1",
					"name": "Deluxe Room",
					"rooms": [
						{ "id": "r1", "name": "Room 101" }
					]
				}
			]
		}
	}`)
	var resp mypms.GetRoomDetailsResponse
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		fmt.Println("Error:", err)
		return
	}
	for _, rt := range resp.RoomTypesList() {
		fmt.Printf("RT %s (Name: %s)\n", rt.ID, rt.Name)
		for _, r := range rt.Rooms {
			fmt.Printf("  Room: %s - %s\n", r.GetID(), r.Name)
		}
	}
}

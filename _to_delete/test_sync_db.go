package main

import (
	"context"
	"fmt"
	"github.com/channel-manager/channel-manager/services/pms/adapters/mypms"
	"github.com/channel-manager/channel-manager/services/pms/usecases"
	"github.com/channel-manager/channel-manager/platform/logger"
	"github.com/channel-manager/channel-manager/platform/auth"
)

func main() {
	// Fake it to test roomTypesFromDetails
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
	rts := mypms.RoomTypesFromDetails("prop-1", &resp)
	fmt.Printf("Parsed RoomTypes: %+v\n", rts)
}

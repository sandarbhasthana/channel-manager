package main

import (
	"encoding/json"
	"fmt"

	"github.com/channel-manager/channel-manager/services/pms/adapters/mypms"
)

func main() {
	payload := []byte(`{
  "data": {
    "room_types": [
      {
        "id": "cmpu45k7v000w9kvsbod2c4l8",
        "name": "Deluxe Room",
        "description": "Spacious deluxe room with premium amenities",
        "rooms": [
          {
            "id": "cmpu45lf8001q9kvsg0sklzfs",
            "name": "Room 202",
            "capacity": 2
          }
        ]
      }
    ]
  },
  "status": 200
}`)

	var resp mypms.GetRoomDetailsResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		fmt.Println("Unmarshal GetRoomDetailsResponse failed:", err)
		return
	}
	
	fmt.Printf("Raw Data length: %d\n", len(resp.Data))

	rts := mypms.TestRoomTypesFromDetails("prop1", &resp)
	for _, rt := range rts {
		fmt.Printf("RT %s (Name: %s)\n", rt.ExternalID, rt.Name)
		fmt.Printf("  Rooms count: %d\n", len(rt.Rooms))
		for _, r := range rt.Rooms {
			fmt.Printf("    -> Room: %s - %s\n", r.ExternalID, r.Name)
		}
	}
}

package postgres

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/channel-manager/channel-manager/services/pms/adapters/postgres/pgstore"
	"github.com/channel-manager/channel-manager/services/pms/domain"
)

func pgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func pgtypeUUID(s string) pgtype.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func propertyToDomain(row pgstore.PmsProperty) domain.Property {
	ext := ""
	if row.ExternalID != nil {
		ext = *row.ExternalID
	}
	connID := ""
	if row.ConnectionID.Valid {
		connID = uuid.UUID(row.ConnectionID.Bytes).String()
	}
	var addr map[string]any
	_ = json.Unmarshal(row.Address, &addr)
	city, country := "", ""
	if addr != nil {
		if v, ok := addr["city"].(string); ok {
			city = v
		}
		if v, ok := addr["country"].(string); ok {
			country = v
		}
	}
	return domain.Property{
		ID:              row.ID.String(),
		OrgID:           row.OrgID.String(),
		ConnectionID:    connID,
		ExternalID:      ext,
		Name:            row.Name,
		Timezone:        row.Timezone,
		DefaultCurrency: row.Currency,
		City:            city,
		Country:         country,
		IsActive:        row.IsActive,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func roomTypeToDomain(row pgstore.PmsRoomType) domain.RoomType {
	ext := ""
	if row.ExternalID != nil {
		ext = *row.ExternalID
	}
	return domain.RoomType{
		ID:            row.ID.String(),
		OrgID:         row.OrgID.String(),
		PropertyID:    row.PropertyID.String(),
		ExternalID:    ext,
		Code:          row.Code,
		Name:          row.Name,
		MaxOccupancy:  int(row.Capacity),
		BaseOccupancy: int(row.BaseOccupancy),
		IsActive:      row.IsActive,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func addressJSON(city, country string) []byte {
	b, _ := json.Marshal(map[string]string{"city": city, "country": country})
	return b
}

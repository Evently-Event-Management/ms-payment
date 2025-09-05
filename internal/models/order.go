package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Order struct {
	bun.BaseModel `bun:"table:orders"`

	OrderID   string    `json:"orderID" bun:"order_id,pk"`
	UserID    string    `json:"userID" bun:"user_id"`
	SessionID string    `json:"sessionID" bun:"session_id"`
	SeatIDs   []string  `json:"seatIDs" bun:"seat_ids,array"`
	Status    string    `json:"status" bun:"status"`
	Price     float64   `json:"price" bun:"price"`
	CreatedAt time.Time `json:"createdAt" bun:"created_at"`
}

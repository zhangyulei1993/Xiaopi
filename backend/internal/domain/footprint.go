package domain

import "time"

type Footprint struct {
	PropertyID string    `json:"propertyId"`
	ViewedAt   time.Time `json:"viewedAt"`
	ViewCount  int       `json:"viewCount"`
}

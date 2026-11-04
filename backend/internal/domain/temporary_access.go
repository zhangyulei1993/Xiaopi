package domain

import "time"

type TemporaryAccess struct {
	ID         uint64    `json:"id"`
	Phone      string    `json:"phone"`
	AccessCode string    `json:"accessCode"`
	Status     string    `json:"status"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedBy  string    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

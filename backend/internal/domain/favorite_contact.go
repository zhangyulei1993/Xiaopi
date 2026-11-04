package domain

import "time"

type FavoriteContact struct {
	UserID       string    `json:"userId"`
	Nickname     string    `json:"nickname"`
	AvatarURL    string    `json:"avatarUrl"`
	Phone        string    `json:"phone"`
	PropertyID   string    `json:"propertyId"`
	PropertyName string    `json:"propertyName"`
	CreatedAt    time.Time `json:"createdAt"`
}

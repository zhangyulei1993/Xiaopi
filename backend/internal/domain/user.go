package domain

import (
	"errors"
	"time"
)

var ErrPhoneInUse = errors.New("phone number is already bound")

type UserStatus string

const (
	UserActive  UserStatus = "active"
	UserBlocked UserStatus = "blocked"
)

type User struct {
	ID        string     `json:"id"`
	OpenID    string     `json:"-"`
	Nickname  string     `json:"nickname"`
	AvatarURL string     `json:"avatarUrl"`
	Phone     string     `json:"phone"`
	Status    UserStatus `json:"status"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type AdminUser struct {
	ID        uint64    `json:"id"`
	UserID    string    `json:"userId"`
	Nickname  string    `json:"nickname"`
	AvatarURL string    `json:"avatarUrl"`
	Phone     string    `json:"phone"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

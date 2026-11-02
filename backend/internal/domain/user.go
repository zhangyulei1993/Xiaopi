package domain

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
}

type Admin struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

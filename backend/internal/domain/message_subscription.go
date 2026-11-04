package domain

type MessageRecipient struct {
	GrantID uint64
	UserID  string
	OpenID  string
	Area    string
}

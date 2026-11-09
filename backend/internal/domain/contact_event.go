package domain

import "time"

// ContactEvent records a lightweight high-intent action without treating it as
// a formal appointment. It lets operators distinguish a consultation or phone
// tap from an appointment that requires a follow-up workflow.
type ContactEvent struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	PropertyID   string    `json:"propertyId"`
	Type         string    `json:"type"`
	Source       string    `json:"source"`
	Nickname     string    `json:"nickname,omitempty"`
	Phone        string    `json:"phone,omitempty"`
	PropertyName string    `json:"propertyName,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

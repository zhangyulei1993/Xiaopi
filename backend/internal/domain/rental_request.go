package domain

import "time"

type RentalRequest struct {
	ID            string    `json:"id"`
	UserID        string    `json:"userId"`
	RequestType   string    `json:"requestType"`
	PropertyID    string    `json:"propertyId,omitempty"`
	ContactName   string    `json:"contactName"`
	Phone         string    `json:"phone"`
	PreferredTime string    `json:"preferredTime"`
	Location      string    `json:"location"`
	Layout        string    `json:"layout"`
	BudgetMin     int64     `json:"budgetMin"`
	BudgetMax     int64     `json:"budgetMax"`
	MoveInDate    string    `json:"moveInDate"`
	Note          string    `json:"note"`
	FollowUpNote  string    `json:"followUpNote"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

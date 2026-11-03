package domain

import "time"

type PropertyLeadStatus string

const (
	LeadNew       PropertyLeadStatus = "new"
	LeadContacted PropertyLeadStatus = "contacted"
	LeadInspect   PropertyLeadStatus = "inspection_pending"
	LeadConverted PropertyLeadStatus = "converted"
	LeadClosed    PropertyLeadStatus = "closed"
)

type PropertyLead struct {
	ID                  string             `json:"id"`
	SourceName          string             `json:"sourceName"`
	SourcePhone         string             `json:"sourcePhone"`
	Community           string             `json:"community"`
	Location            string             `json:"location"`
	Layout              string             `json:"layout"`
	ExpectedRentMonthly int64              `json:"expectedRentMonthly"`
	Note                string             `json:"note"`
	FollowUpNote        string             `json:"followUpNote"`
	Status              PropertyLeadStatus `json:"status"`
	ConvertedPropertyID string             `json:"convertedPropertyId,omitempty"`
	CreatedAt           time.Time          `json:"createdAt"`
	UpdatedAt           time.Time          `json:"updatedAt"`
}

type PropertyStatusChange struct {
	ID         string    `json:"id"`
	PropertyID string    `json:"propertyId"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	OperatorID string    `json:"operatorId"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"createdAt"`
}

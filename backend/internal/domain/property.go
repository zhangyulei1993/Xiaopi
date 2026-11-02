package domain

import "time"

type PropertyStatus string

const (
	PropertyDraft     PropertyStatus = "draft"
	PropertyPending   PropertyStatus = "pending_review"
	PropertyPublished PropertyStatus = "published"
	PropertyOffline   PropertyStatus = "offline"
)

type Property struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Community   string          `json:"community"`
	Location    string          `json:"location"`
	Layout      string          `json:"layout"`
	RentMonthly int64           `json:"rentMonthly"`
	FeeText     string          `json:"feeText"`
	Description string          `json:"description"`
	Status      PropertyStatus  `json:"status"`
	OwnerID     string          `json:"ownerId"`
	Media       []PropertyMedia `json:"media"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type PropertyMedia struct {
	ID         string `json:"id"`
	PropertyID string `json:"propertyId"`
	Type       string `json:"type"`
	URL        string `json:"url"`
	Sort       int    `json:"sort"`
}

func (p *Property) SubmitForReview() {
	if p.Status == PropertyDraft || p.Status == PropertyOffline {
		p.Status = PropertyPending
		p.UpdatedAt = time.Now()
	}
}

func (p *Property) Publish() {
	if p.Status == PropertyPending {
		p.Status = PropertyPublished
		p.UpdatedAt = time.Now()
	}
}

func (p *Property) TakeOffline() {
	if p.Status == PropertyPublished {
		p.Status = PropertyOffline
		p.UpdatedAt = time.Now()
	}
}

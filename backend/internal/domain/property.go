package domain

import (
	"errors"
	"time"
)

var ErrInvalidPropertyStatusTransition = errors.New("invalid property status transition")

type PropertyStatus string

const (
	PropertyDraft     PropertyStatus = "draft"
	PropertyPending   PropertyStatus = "pending_review"
	PropertyPublished PropertyStatus = "published"
	PropertyAvailable PropertyStatus = "available"
	PropertyRented    PropertyStatus = "rented"
	PropertyOffline   PropertyStatus = "offline"
)

type Property struct {
	ID                   string          `json:"id"`
	Title                string          `json:"title"`
	Community            string          `json:"community"`
	Location             string          `json:"location"`
	AreaLabel            string          `json:"areaLabel"`
	BuildingNo           string          `json:"buildingNo"`
	UnitNo               string          `json:"unitNo"`
	RoomNo               string          `json:"roomNo"`
	Latitude             float64         `json:"latitude"`
	Longitude            float64         `json:"longitude"`
	Layout               string          `json:"layout"`
	RentMonthly          int64           `json:"rentMonthly"`
	FeeText              string          `json:"feeText"`
	RentType             string          `json:"rentType"`
	ListingSource        string          `json:"listingSource"`
	RemainingLeaseMonths int             `json:"remainingLeaseMonths"`
	TransferDeposit      int64           `json:"transferDeposit"`
	AvailableDate        string          `json:"availableDate"`
	DepositPayment       string          `json:"depositPayment"`
	UtilitiesText        string          `json:"utilitiesText"`
	AreaSqm              int             `json:"areaSqm"`
	FloorInfo            string          `json:"floorInfo"`
	Orientation          string          `json:"orientation"`
	Elevator             string          `json:"elevator"`
	MetroMinutes         int             `json:"metroMinutes"`
	VerificationDate     string          `json:"verificationDate"`
	RulesText            string          `json:"rulesText"`
	Description          string          `json:"description"`
	DisplayTags          []string        `json:"displayTags"`
	Status               PropertyStatus  `json:"status"`
	OwnerID              string          `json:"ownerId"`
	Media                []PropertyMedia `json:"media"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
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

func IsPropertyStatus(value string) bool {
	switch PropertyStatus(value) {
	case PropertyDraft, PropertyPending, PropertyPublished, PropertyAvailable, PropertyRented, PropertyOffline:
		return true
	default:
		return false
	}
}

func CanTransitionPropertyStatus(from, to string) bool {
	if !IsPropertyStatus(from) || !IsPropertyStatus(to) || from == to {
		return false
	}
	switch PropertyStatus(from) {
	case PropertyDraft, PropertyPending:
		// A single operator may publish directly after previewing the draft.
		return PropertyStatus(to) == PropertyPublished || PropertyStatus(to) == PropertyAvailable
	case PropertyPublished:
		// Kept for listings created before the simplified workflow.
		return PropertyStatus(to) == PropertyAvailable || PropertyStatus(to) == PropertyRented || PropertyStatus(to) == PropertyOffline
	case PropertyAvailable:
		return PropertyStatus(to) == PropertyRented || PropertyStatus(to) == PropertyOffline
	case PropertyRented:
		return PropertyStatus(to) == PropertyAvailable
	case PropertyOffline:
		return PropertyStatus(to) == PropertyAvailable || PropertyStatus(to) == PropertyRented
	default:
		return false
	}
}

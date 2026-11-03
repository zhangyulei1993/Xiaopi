package domain

import "time"

type OperationLog struct {
	ID         string    `json:"id"`
	OperatorID string    `json:"operatorId"`
	TargetType string    `json:"targetType"`
	TargetID   string    `json:"targetId"`
	Action     string    `json:"action"`
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"createdAt"`
}

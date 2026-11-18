package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

var ErrConfigConflict = errors.New("configuration changed since it was loaded")

type ConfigPublication struct {
	Status       string       `json:"status,omitempty"`
	Error        string       `json:"error,omitempty"`
	ID           string       `json:"id"`
	Operator     string       `json:"operator"`
	CreatedAt    time.Time    `json:"createdAt"`
	Revision     string       `json:"revision"`
	RestoredFrom string       `json:"restoredFrom,omitempty"`
	Before       PublicConfig `json:"before"`
	After        PublicConfig `json:"after"`
}

func ConfigRevision(config PublicConfig) string {
	config.Normalize()
	data, _ := json.Marshal(config)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

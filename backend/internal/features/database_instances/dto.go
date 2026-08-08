package database_instances

import "time"

type DiscoveredDatabase struct {
	Name           string     `json:"name"`
	SizeMb         *float64   `json:"sizeMb,omitempty"`
	Owner          *string    `json:"owner,omitempty"`
	IsConfigured   bool       `json:"isConfigured"`
	LastBackupTime *time.Time `json:"lastBackupTime,omitzero"`
}

type DiscoverDatabasesResponse struct {
	Databases    []DiscoveredDatabase `json:"databases"`
	DiscoveredAt time.Time            `json:"discoveredAt"`
}

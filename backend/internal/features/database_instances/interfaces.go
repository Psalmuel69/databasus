package database_instances

import (
	"context"

	"databasus-backend/internal/util/encryption"
)

// DiscoveryProvider lists every database visible on a connected instance for one
// engine family. Name, size and owner come back in a single round trip so a fleet
// of thousands of databases costs one query, not one query per database.
type DiscoveryProvider interface {
	DiscoverDatabases(
		ctx context.Context,
		instance *DatabaseInstance,
		encryptor encryption.FieldEncryptor,
	) ([]DiscoveredDatabase, error)
}

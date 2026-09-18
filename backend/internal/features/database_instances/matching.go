package database_instances

import (
	"context"
	"strings"
	"time"

	"databasus-backend/internal/features/databases"
	users_models "databasus-backend/internal/features/users/models"
)

type configuredDatabaseInfo struct {
	lastBackupTime *time.Time
}

// findConfiguredDatabases returns, keyed by database name, every existing
// Database row in the workspace that points at the same server as the instance
// (matching engine family, host, and port). One workspace-wide query with
// preloads - no per-name lookups.
func (s *DatabaseInstanceService) findConfiguredDatabases(
	ctx context.Context,
	user *users_models.User,
	instance *DatabaseInstance,
) (map[string]configuredDatabaseInfo, error) {
	existing, err := s.databaseService.GetDatabasesByWorkspace(ctx, user, instance.WorkspaceID)
	if err != nil {
		return nil, err
	}

	configured := make(map[string]configuredDatabaseInfo)

	for _, database := range existing {
		name, isMatch := matchDatabaseToInstance(database, instance)
		if !isMatch {
			continue
		}

		configured[name] = configuredDatabaseInfo{lastBackupTime: database.LastBackupTime}
	}

	return configured, nil
}

// matchDatabaseToInstance reports whether an existing Database row lives on the
// given instance, and if so which server-side database name it manages.
func matchDatabaseToInstance(
	database *databases.Database,
	instance *DatabaseInstance,
) (string, bool) {
	switch instance.Type {
	case InstanceTypePostgres:
		if pg := database.PostgresqlLogical; pg != nil && pg.Database != nil {
			return *pg.Database, hostPortMatches(pg.Host, &pg.Port, instance)
		}

		// Physical Postgres registrations have no per-database name - they
		// back up the whole server - so they're matched by host/port alone
		// and reported under the parent Database row's own name.
		if phys := database.PostgresqlPhysical; phys != nil {
			return database.Name, hostPortMatches(phys.Host, &phys.Port, instance)
		}

		return "", false
	case InstanceTypeMysql:
		my := database.Mysql
		if my == nil || my.Database == nil {
			return "", false
		}

		return *my.Database, hostPortMatches(my.Host, &my.Port, instance)
	case InstanceTypeMariadb:
		maria := database.Mariadb
		if maria == nil || maria.Database == nil {
			return "", false
		}

		return *maria.Database, hostPortMatches(maria.Host, &maria.Port, instance)
	case InstanceTypeMongodb:
		mongo := database.Mongodb
		if mongo == nil || mongo.Database == "" {
			return "", false
		}

		return mongo.Database, mongoHostPortMatches(mongo.Host, mongo.Port, instance)
	default:
		return "", false
	}
}

func hostPortMatches(host string, port *int, instance *DatabaseInstance) bool {
	if !strings.EqualFold(strings.TrimSpace(host), strings.TrimSpace(instance.Host)) {
		return false
	}

	if instance.Port == nil || port == nil {
		return instance.Port == nil && port == nil
	}

	return *port == *instance.Port
}

func mongoHostPortMatches(host string, port *int, instance *DatabaseInstance) bool {
	if !strings.EqualFold(strings.TrimSpace(host), strings.TrimSpace(instance.Host)) {
		return false
	}

	// SRV connections have no port on either side.
	if instance.IsSrv {
		return true
	}

	return hostPortMatches(host, port, instance)
}

package database_instances

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	postgresql_shared "databasus-backend/internal/features/databases/databases/postgresql/shared"
	"databasus-backend/internal/util/encryption"
)

// postgresMaintenanceDatabase is the database every PostgreSQL server ships with,
// used purely as a connection target to query the server-wide pg_database catalog.
const postgresMaintenanceDatabase = "postgres"

type PostgresDiscoveryProvider struct{}

func (p *PostgresDiscoveryProvider) DiscoverDatabases(
	ctx context.Context,
	instance *DatabaseInstance,
	encryptor encryption.FieldEncryptor,
) ([]DiscoveredDatabase, error) {
	conn, closeConn, err := openPostgresInstanceConn(ctx, instance, encryptor)
	if err != nil {
		return nil, err
	}
	defer closeConn()

	rows, err := conn.Query(ctx, `
		SELECT
			d.datname,
			pg_catalog.pg_database_size(d.datname),
			pg_catalog.pg_get_userbyid(d.datdba)
		FROM pg_catalog.pg_database d
		WHERE d.datistemplate = false
		  AND d.datallowconn = true
		  AND pg_catalog.has_database_privilege(d.datname, 'CONNECT')
		ORDER BY d.datname
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query pg_database: %w", err)
	}
	defer rows.Close()

	var databases []DiscoveredDatabase

	for rows.Next() {
		var name, owner string

		var sizeBytes int64

		if err := rows.Scan(&name, &sizeBytes, &owner); err != nil {
			return nil, fmt.Errorf("failed to scan pg_database row: %w", err)
		}

		sizeMb := float64(sizeBytes) / (1024 * 1024)

		databases = append(databases, DiscoveredDatabase{Name: name, SizeMb: &sizeMb, Owner: &owner})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read pg_database rows: %w", err)
	}

	return databases, nil
}

func openPostgresInstanceConn(
	ctx context.Context,
	instance *DatabaseInstance,
	encryptor encryption.FieldEncryptor,
) (*pgx.Conn, func(), error) {
	if instance.Port == nil {
		return nil, nil, errors.New("port is required for PostgreSQL instances")
	}

	spec := postgresql_shared.CredentialSpec{
		Host:          instance.Host,
		Port:          *instance.Port,
		Username:      instance.Username,
		SslMode:       instance.SslMode,
		SslClientCert: instance.SslClientCert,
		SslClientKey:  instance.SslClientKey,
		SslRootCert:   instance.SslRootCert,
	}

	password, err := postgresql_shared.DecryptFieldIfNeeded(instance.Password, encryptor)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt password: %w", err)
	}

	files, err := postgresql_shared.WriteCredentialFilesToTempDir(spec, password, encryptor)
	if err != nil {
		return nil, nil, err
	}

	connString := postgresql_shared.BuildConnString(spec, password, postgresMaintenanceDatabase, files)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		files.Remove()

		return nil, nil, fmt.Errorf("failed to connect to PostgreSQL instance: %w", err)
	}

	closeConn := func() {
		_ = conn.Close(ctx)
		files.Remove()
	}

	return conn, closeConn, nil
}

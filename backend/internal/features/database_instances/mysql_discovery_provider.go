package database_instances

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"

	"databasus-backend/internal/util/encryption"
)

var mysqlSystemSchemas = []string{"information_schema", "mysql", "performance_schema", "sys"}

type MysqlDiscoveryProvider struct{}

func (p *MysqlDiscoveryProvider) DiscoverDatabases(
	ctx context.Context,
	instance *DatabaseInstance,
	encryptor encryption.FieldEncryptor,
) ([]DiscoveredDatabase, error) {
	dsn, err := buildMysqlInstanceDSN(instance, encryptor)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to %s instance: %w", instance.Type, err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT
			s.schema_name,
			COALESCE(SUM(t.data_length + t.index_length), 0)
		FROM information_schema.schemata s
		LEFT JOIN information_schema.tables t ON t.table_schema = s.schema_name
		WHERE s.schema_name NOT IN (?, ?, ?, ?)
		GROUP BY s.schema_name
		ORDER BY s.schema_name
	`, mysqlSystemSchemas[0], mysqlSystemSchemas[1], mysqlSystemSchemas[2], mysqlSystemSchemas[3])
	if err != nil {
		return nil, fmt.Errorf("failed to query information_schema: %w", err)
	}
	defer rows.Close()

	var databases []DiscoveredDatabase

	for rows.Next() {
		var name string

		var sizeBytes int64

		if err := rows.Scan(&name, &sizeBytes); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}

		sizeMb := float64(sizeBytes) / (1024 * 1024)

		databases = append(databases, DiscoveredDatabase{Name: name, SizeMb: &sizeMb})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read schema rows: %w", err)
	}

	return databases, nil
}

func buildMysqlInstanceDSN(instance *DatabaseInstance, encryptor encryption.FieldEncryptor) (string, error) {
	if instance.Port == nil {
		return "", fmt.Errorf("port is required for %s instances", instance.Type)
	}

	password, err := decryptFieldIfNeeded(instance.Password, encryptor)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt password: %w", err)
	}

	tlsConfig := "false"

	if instance.IsTlsEnabled {
		// Only errors when the key collides with a driver-reserved name, which
		// "mysql-skip-verify" never does - safe to ignore, same as the mysql backup provider.
		_ = mysqldriver.RegisterTLSConfig("mysql-skip-verify", &tls.Config{InsecureSkipVerify: true})

		tlsConfig = "mysql-skip-verify"
	}

	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/?parseTime=true&timeout=15s&tls=%s&charset=utf8mb4",
		instance.Username,
		password,
		instance.Host,
		*instance.Port,
		tlsConfig,
	), nil
}

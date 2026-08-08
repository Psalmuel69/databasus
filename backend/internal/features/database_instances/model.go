package database_instances

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	postgresql_shared "databasus-backend/internal/features/databases/databases/postgresql/shared"
	"databasus-backend/internal/util/encryption"
)

type DatabaseInstance struct {
	ID          uuid.UUID    `json:"id"          gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	WorkspaceID uuid.UUID    `json:"workspaceId" gorm:"column:workspace_id;type:uuid;not null"`
	Name        string       `json:"name"        gorm:"column:name;type:text;not null"`
	Type        InstanceType `json:"type"        gorm:"column:type;type:text;not null"`

	Host     string `json:"host"     gorm:"column:host;type:text;not null"`
	Port     *int   `json:"port"     gorm:"column:port;type:int"`
	Username string `json:"username" gorm:"column:username;type:text;not null"`
	Password string `json:"password" gorm:"column:password;type:text;not null"`

	// PostgreSQL only
	SslMode       postgresql_shared.PostgresSslMode `json:"sslMode,omitempty"       gorm:"column:ssl_mode;type:text;not null;default:'disable'"`
	SslClientCert string                            `json:"sslClientCert,omitempty" gorm:"column:ssl_client_cert;type:text;not null;default:''"`
	SslClientKey  string                            `json:"sslClientKey,omitempty"  gorm:"column:ssl_client_key;type:text;not null;default:''"`
	SslRootCert   string                            `json:"sslRootCert,omitempty"   gorm:"column:ssl_root_cert;type:text;not null;default:''"`

	// MySQL and MariaDB only
	IsTlsEnabled bool `json:"isTlsEnabled,omitempty" gorm:"column:is_tls_enabled;type:boolean;not null;default:false"`

	// MongoDB only
	AuthDatabase string `json:"authDatabase,omitempty" gorm:"column:auth_database;type:text;not null;default:'admin'"`
	IsSrv        bool   `json:"isSrv,omitempty"        gorm:"column:is_srv;type:boolean;not null;default:false"`

	LastDiscoveredAt *time.Time `json:"lastDiscoveredAt,omitzero" gorm:"column:last_discovered_at;type:timestamptz"`
	CreatedAt        time.Time  `json:"createdAt"                 gorm:"column:created_at;type:timestamptz;not null;default:now()"`
}

func (DatabaseInstance) TableName() string {
	return "database_instances"
}

func (i *DatabaseInstance) Validate() error {
	if i.Name == "" {
		return errors.New("name is required")
	}

	if i.Host == "" {
		return errors.New("host is required")
	}

	if i.Username == "" {
		return errors.New("username is required")
	}

	if i.Password == "" {
		return errors.New("password is required")
	}

	switch i.Type {
	case InstanceTypePostgres:
		if i.Port == nil || *i.Port == 0 {
			return errors.New("port is required")
		}

		if i.SslMode == "" {
			i.SslMode = postgresql_shared.PostgresSslModeDisable
		}

		return postgresql_shared.ValidateSslConfig(i.SslMode, i.SslClientCert, i.SslClientKey, i.SslRootCert)
	case InstanceTypeMysql, InstanceTypeMariadb:
		if i.Port == nil || *i.Port == 0 {
			return errors.New("port is required")
		}

		return nil
	case InstanceTypeMongodb:
		if !i.IsSrv && (i.Port == nil || *i.Port == 0) {
			return errors.New("port is required for standard (non-SRV) connections")
		}

		if i.AuthDatabase == "" {
			i.AuthDatabase = "admin"
		}

		return nil
	default:
		return fmt.Errorf("invalid instance type: %q", i.Type)
	}
}

func (i *DatabaseInstance) Update(incoming *DatabaseInstance) {
	i.Name = incoming.Name
	i.Host = incoming.Host
	i.Port = incoming.Port
	i.Username = incoming.Username
	i.SslMode = incoming.SslMode
	i.SslClientCert = incoming.SslClientCert
	i.SslRootCert = incoming.SslRootCert
	i.IsTlsEnabled = incoming.IsTlsEnabled
	i.AuthDatabase = incoming.AuthDatabase
	i.IsSrv = incoming.IsSrv

	if incoming.Password != "" {
		i.Password = incoming.Password
	}

	if incoming.SslClientKey != "" {
		i.SslClientKey = incoming.SslClientKey
	}
}

func (i *DatabaseInstance) EncryptSensitiveFields(encryptor encryption.FieldEncryptor) error {
	for _, field := range []*string{&i.Password, &i.SslClientCert, &i.SslClientKey, &i.SslRootCert} {
		if *field == "" {
			continue
		}

		encrypted, err := encryptor.Encrypt(*field)
		if err != nil {
			return err
		}

		*field = encrypted
	}

	return nil
}

func (i *DatabaseInstance) HideSensitiveData() {
	if i == nil {
		return
	}

	i.Password = ""
	i.SslClientKey = ""
}

func decryptFieldIfNeeded(value string, encryptor encryption.FieldEncryptor) (string, error) {
	if encryptor == nil {
		return value, nil
	}

	return encryptor.Decrypt(value)
}

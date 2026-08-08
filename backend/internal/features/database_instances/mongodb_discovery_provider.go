package database_instances

import (
	"context"
	"fmt"
	"net/url"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"databasus-backend/internal/util/encryption"
)

var mongodbSystemDatabases = map[string]bool{"admin": true, "local": true, "config": true}

type MongodbDiscoveryProvider struct{}

func (p *MongodbDiscoveryProvider) DiscoverDatabases(
	ctx context.Context,
	instance *DatabaseInstance,
	encryptor encryption.FieldEncryptor,
) ([]DiscoveredDatabase, error) {
	uri, err := buildMongodbInstanceURI(instance, encryptor)
	if err != nil {
		return nil, err
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB instance: %w", err)
	}
	defer func() { _ = client.Disconnect(ctx) }()

	// AuthorizedDatabases limits results to what this user can actually see,
	// so discovery never exposes inaccessible databases.
	authorizedOnly := true

	result, err := client.ListDatabases(
		ctx,
		bson.D{},
		&options.ListDatabasesOptions{AuthorizedDatabases: &authorizedOnly},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list MongoDB databases: %w", err)
	}

	var databases []DiscoveredDatabase

	for _, spec := range result.Databases {
		if mongodbSystemDatabases[spec.Name] {
			continue
		}

		sizeMb := float64(spec.SizeOnDisk) / (1024 * 1024)

		databases = append(databases, DiscoveredDatabase{Name: spec.Name, SizeMb: &sizeMb})
	}

	return databases, nil
}

func buildMongodbInstanceURI(instance *DatabaseInstance, encryptor encryption.FieldEncryptor) (string, error) {
	password, err := decryptFieldIfNeeded(instance.Password, encryptor)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt password: %w", err)
	}

	authDB := instance.AuthDatabase
	if authDB == "" {
		authDB = "admin"
	}

	if instance.IsSrv {
		return fmt.Sprintf(
			"mongodb+srv://%s:%s@%s/?authSource=%s&connectTimeoutMS=15000",
			url.QueryEscape(instance.Username),
			url.QueryEscape(password),
			instance.Host,
			url.QueryEscape(authDB),
		), nil
	}

	port := 27017
	if instance.Port != nil {
		port = *instance.Port
	}

	return fmt.Sprintf(
		"mongodb://%s:%s@%s:%d/?authSource=%s&connectTimeoutMS=15000",
		url.QueryEscape(instance.Username),
		url.QueryEscape(password),
		instance.Host,
		port,
		url.QueryEscape(authDB),
	), nil
}

package database_instances

import "fmt"

// MariaDB speaks MySQL's wire protocol and information_schema, so it reuses
// MysqlDiscoveryProvider rather than duplicating it.
var discoveryProviders = map[InstanceType]DiscoveryProvider{
	InstanceTypePostgres: &PostgresDiscoveryProvider{},
	InstanceTypeMysql:    &MysqlDiscoveryProvider{},
	InstanceTypeMariadb:  &MysqlDiscoveryProvider{},
	InstanceTypeMongodb:  &MongodbDiscoveryProvider{},
}

func GetDiscoveryProvider(instanceType InstanceType) (DiscoveryProvider, error) {
	provider, isOk := discoveryProviders[instanceType]
	if !isOk {
		return nil, fmt.Errorf("no discovery provider registered for instance type: %q", instanceType)
	}

	return provider, nil
}

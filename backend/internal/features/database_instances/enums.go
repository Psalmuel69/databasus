package database_instances

type InstanceType string

const (
	InstanceTypePostgres InstanceType = "POSTGRES"
	InstanceTypeMysql    InstanceType = "MYSQL"
	InstanceTypeMariadb  InstanceType = "MARIADB"
	InstanceTypeMongodb  InstanceType = "MONGODB"
)

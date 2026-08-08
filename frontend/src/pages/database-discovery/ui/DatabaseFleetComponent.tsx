import { App, Spin } from 'antd';
import { type JSX, useEffect, useState } from 'react';

import {
  DatabaseType,
  getDatabaseLogoFromType,
  getDatabaseTypeLabel,
} from '../../../entity/databases';
import { databaseInstancesApi } from '../api/databaseInstancesApi';
import type { DatabaseInstance } from '../model/DatabaseInstance';
import type { DiscoveredInstance } from '../model/DiscoveredInstance';
import type { FleetDatabase } from '../model/FleetDatabase';
import { InstanceType } from '../model/InstanceType';
import { mapDiscoveredToFleetDatabases } from '../model/mapDiscoveredToFleetDatabases';
import { DatabaseDiscoveryPageComponent } from './DatabaseDiscoveryPageComponent';
import { RegisterInstanceComponent } from './RegisterInstanceComponent';

interface Props {
  workspaceId: string;
}

const LOGO_TYPE_BY_INSTANCE: Record<InstanceType, DatabaseType> = {
  [InstanceType.POSTGRES]: DatabaseType.POSTGRES_LOGICAL,
  [InstanceType.MYSQL]: DatabaseType.MYSQL,
  [InstanceType.MARIADB]: DatabaseType.MARIADB,
  [InstanceType.MONGODB]: DatabaseType.MONGODB,
};

const toDiscoveredInstance = (instance: DatabaseInstance): DiscoveredInstance => {
  const logoType = LOGO_TYPE_BY_INSTANCE[instance.type];

  return {
    host: instance.host,
    port: instance.port ?? 0,
    engineLabel: getDatabaseTypeLabel(logoType),
    engineIconSrc: getDatabaseLogoFromType(logoType),
    lastDiscoveredAt: instance.lastDiscoveredAt ? new Date(instance.lastDiscoveredAt) : new Date(),
  };
};

export const DatabaseFleetComponent = ({ workspaceId }: Props): JSX.Element => {
  const { message } = App.useApp();
  const [isLoading, setIsLoading] = useState(true);
  const [instance, setInstance] = useState<DatabaseInstance>();
  const [fleetDatabases, setFleetDatabases] = useState<FleetDatabase[]>();
  const [isEditing, setIsEditing] = useState(false);

  const discoverInstanceDatabases = async (targetInstance: DatabaseInstance) => {
    if (!targetInstance.id) return;

    try {
      const response = await databaseInstancesApi.discoverDatabases(targetInstance.id);
      setFleetDatabases(mapDiscoveredToFleetDatabases(response.databases));
    } catch (error) {
      message.error((error as Error).message || 'Discovery failed');
      setFleetDatabases([]);
    }
  };

  const loadInstances = async () => {
    setIsLoading(true);

    try {
      const instances = await databaseInstancesApi.getInstances(workspaceId);

      if (instances.length > 0) {
        setInstance(instances[0]);
        await discoverInstanceDatabases(instances[0]);
      }
    } catch (error) {
      message.error((error as Error).message || 'Failed to load instances');
    }

    setIsLoading(false);
  };

  const handleInstanceSaved = async (saved: DatabaseInstance) => {
    setInstance(saved);
    setIsEditing(false);
    setIsLoading(true);
    await discoverInstanceDatabases(saved);
    setIsLoading(false);
  };

  const handleDeleteInstance = async () => {
    if (!instance?.id) return;

    try {
      await databaseInstancesApi.deleteInstance(instance.id);
      message.success(`Instance "${instance.name}" removed`);
      setInstance(undefined);
      setFleetDatabases(undefined);
    } catch (error) {
      message.error((error as Error).message || 'Failed to remove instance');
    }
  };

  const refreshDiscovery = async (): Promise<FleetDatabase[]> => {
    if (!instance?.id) return fleetDatabases ?? [];

    const response = await databaseInstancesApi.discoverDatabases(instance.id);
    const refreshed = mapDiscoveredToFleetDatabases(response.databases);
    setFleetDatabases(refreshed);

    return refreshed;
  };

  useEffect(() => {
    loadInstances();
  }, [workspaceId]);

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Spin />
      </div>
    );
  }

  if (!instance || isEditing) {
    return (
      <RegisterInstanceComponent
        workspaceId={workspaceId}
        existingInstance={isEditing ? instance : undefined}
        onSaved={handleInstanceSaved}
        onCancel={isEditing ? () => setIsEditing(false) : undefined}
      />
    );
  }

  return (
    <DatabaseDiscoveryPageComponent
      key={instance.id}
      instance={toDiscoveredInstance(instance)}
      initialDatabases={fleetDatabases ?? []}
      onRefreshDiscovery={refreshDiscovery}
      workspaceId={workspaceId}
      instanceId={instance.id}
      instanceType={instance.type}
      onEditInstance={() => setIsEditing(true)}
      onDeleteInstance={handleDeleteInstance}
    />
  );
};

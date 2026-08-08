import type { DiscoveredDatabaseDto } from './DiscoveredDatabaseDto';
import { DiscoveryStatus } from './DiscoveryStatus';
import type { FleetDatabase } from './FleetDatabase';

export const mapDiscoveredToFleetDatabases = (
  discovered: DiscoveredDatabaseDto[] | null,
): FleetDatabase[] =>
  (discovered ?? []).map((dto) => ({
    id: dto.name,
    name: dto.name,
    status: dto.isConfigured ? DiscoveryStatus.CONFIGURED : DiscoveryStatus.DISCOVERED,
    sizeMb: dto.sizeMb,
    owner: dto.owner,
    lastBackupTime: dto.lastBackupTime ? new Date(dto.lastBackupTime) : undefined,
  }));

import type { DiscoveryStatus } from './DiscoveryStatus';

export interface FleetDatabase {
  id: string;
  name: string;
  status: DiscoveryStatus;
  sizeMb?: number;
  owner?: string;
  lastBackupTime?: Date;
  recoveryModel?: string;
}

import type { LogicalBackupConfig } from '../../../entity/backups/logical';
import type { PhysicalBackupConfig } from '../../../entity/backups/physical';
import type { PhysicalDatabaseBackupType } from '../../../entity/databases';
import type { BulkBackupType } from './BulkBackupType';

export interface BulkConfigureBackupsRequest {
  instanceId: string;
  databaseNames: string[];

  backupType: BulkBackupType;

  // Exactly one of these is set, matching backupType.
  logicalConfig?: Omit<LogicalBackupConfig, 'databaseId'>;
  physicalConfig?: Omit<PhysicalBackupConfig, 'databaseId'>;

  // Only used when backupType is PHYSICAL - the database-level strategy that
  // decides which retention modes physicalConfig may use.
  physicalBackupType?: PhysicalDatabaseBackupType;

  notifierIds: string[];
}

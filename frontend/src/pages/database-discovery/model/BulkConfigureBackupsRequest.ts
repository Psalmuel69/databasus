import type { LogicalBackupConfig } from '../../../entity/backups/logical';

export interface BulkConfigureBackupsRequest {
  instanceId: string;
  databaseNames: string[];
  backupConfig: Omit<LogicalBackupConfig, 'databaseId'>;
}

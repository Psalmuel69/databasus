export interface DiscoveredDatabaseDto {
  name: string;
  sizeMb?: number;
  owner?: string;
  isConfigured: boolean;
  lastBackupTime?: string;
}

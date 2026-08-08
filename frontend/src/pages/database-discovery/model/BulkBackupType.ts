// Mirrors the backend's BulkBackupType. PHYSICAL is only valid when the
// parent instance is PostgreSQL - physical backups capture the whole server,
// not a single database, so choosing it against multiple selected databases
// creates one independent whole-server backup job per selection.
export enum BulkBackupType {
  LOGICAL = 'LOGICAL',
  PHYSICAL = 'PHYSICAL',
}

import type { BulkConfigureFailure } from './BulkConfigureFailure';

export interface BulkConfigureBackupsResponse {
  createdCount: number;
  skipped: string[];
  failed: BulkConfigureFailure[];
}

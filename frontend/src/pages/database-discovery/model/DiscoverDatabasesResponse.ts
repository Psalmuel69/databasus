import type { DiscoveredDatabaseDto } from './DiscoveredDatabaseDto';

export interface DiscoverDatabasesResponse {
  databases: DiscoveredDatabaseDto[] | null;
  discoveredAt: string;
}

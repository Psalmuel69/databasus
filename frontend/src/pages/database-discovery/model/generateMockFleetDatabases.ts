import { DiscoveryStatus } from './DiscoveryStatus';
import type { FleetDatabase } from './FleetDatabase';

const OWNERS = ['app_svc', 'reporting_ro', 'etl_pipeline', 'finance_admin', 'analytics_ro'];

const RECOVERY_MODELS = ['Full', 'Simple', 'Bulk-logged'];

const NAME_GROUPS: { prefix: string; count: number }[] = [
  { prefix: 'customer_', count: 640 },
  { prefix: 'finance_', count: 85 },
  { prefix: 'tenant_', count: 310 },
  { prefix: 'audit_', count: 24 },
];

const FIXED_NAMES = ['accounting', 'payments', 'billing_archive', 'staging_snapshot'];

// Deterministic PRNG so the mock fleet is stable across renders and reloads.
const createSeededRandom = (seed: number) => {
  let state = seed;

  return () => {
    state = (state * 1103515245 + 12345) & 0x7fffffff;
    return state / 0x7fffffff;
  };
};

const pickStatus = (random: () => number): DiscoveryStatus => {
  const roll = random();

  if (roll < 0.62) return DiscoveryStatus.CONFIGURED;
  if (roll < 0.9) return DiscoveryStatus.DISCOVERED;
  if (roll < 0.97) return DiscoveryStatus.OFFLINE;
  return DiscoveryStatus.INACCESSIBLE;
};

const buildDatabase = (name: string, random: () => number): FleetDatabase => {
  const status = pickStatus(random);
  const isAccessible = status !== DiscoveryStatus.INACCESSIBLE;

  const lastBackupTime =
    status === DiscoveryStatus.CONFIGURED
      ? new Date(Date.now() - Math.floor(random() * 72) * 60 * 60_000)
      : undefined;

  return {
    id: name,
    name,
    status,
    sizeMb: isAccessible ? Math.floor(random() * 240_000) + 40 : undefined,
    owner: isAccessible ? OWNERS[Math.floor(random() * OWNERS.length)] : undefined,
    lastBackupTime,
    recoveryModel: isAccessible
      ? RECOVERY_MODELS[Math.floor(random() * RECOVERY_MODELS.length)]
      : undefined,
  };
};

export const generateMockFleetDatabases = (seed = 1): FleetDatabase[] => {
  const random = createSeededRandom(seed);
  const databases: FleetDatabase[] = [];

  for (const group of NAME_GROUPS) {
    for (let index = 1; index <= group.count; index += 1) {
      const suffix = String(index).padStart(3, '0');
      databases.push(buildDatabase(`${group.prefix}${suffix}`, random));
    }
  }

  for (const name of FIXED_NAMES) {
    databases.push(buildDatabase(name, random));
  }

  return databases;
};

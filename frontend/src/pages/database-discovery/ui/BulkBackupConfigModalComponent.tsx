import { InfoCircleOutlined } from '@ant-design/icons';
import { App, Button, Modal, Radio, Tooltip } from 'antd';
import { type JSX, useState } from 'react';

import type { LogicalBackupConfig } from '../../../entity/backups/logical';
import type { PhysicalBackupConfig } from '../../../entity/backups/physical';
import {
  type Database,
  DatabaseType,
  PhysicalDatabaseBackupType,
  PostgresSslMode,
  initializeDatabaseTypeData,
} from '../../../entity/databases';
import type { PostgresqlPhysicalDatabase } from '../../../entity/databases/model/postgresql/physical/PostgresqlPhysicalDatabase';
import type { Notifier } from '../../../entity/notifiers';
import { EditLogicalBackupConfigComponent } from '../../../features/backups/logical';
import { EditPhysicalBackupConfigComponent } from '../../../features/backups/physical';
import { EditDatabaseNotifiersComponent } from '../../../features/databases/ui/edit/EditDatabaseNotifiersComponent';
import { databaseInstancesApi } from '../api/databaseInstancesApi';
import { BulkBackupType } from '../model/BulkBackupType';
import type { BulkConfigureBackupsRequest } from '../model/BulkConfigureBackupsRequest';
import { InstanceType } from '../model/InstanceType';

interface Props {
  selectedCount: number;
  open: boolean;
  onClose: () => void;
  onConfigured: () => void;

  workspaceId: string;
  instanceId: string;
  instanceType: InstanceType;
  selectedNames: string[];
}

type WizardStep = 'backup-type' | 'backup-config' | 'notifiers';

const ENGINE_TO_LOGICAL_DATABASE_TYPE: Record<InstanceType, DatabaseType> = {
  [InstanceType.POSTGRES]: DatabaseType.POSTGRES_LOGICAL,
  [InstanceType.MYSQL]: DatabaseType.MYSQL,
  [InstanceType.MARIADB]: DatabaseType.MARIADB,
  [InstanceType.MONGODB]: DatabaseType.MONGODB,
};

const physicalBackupTypeOptions = [
  {
    label: 'Full backups only',
    value: PhysicalDatabaseBackupType.FULL,
    description: 'Periodic standalone full backups. Each backup is self-contained.',
  },
  {
    label: 'Full + incremental',
    value: PhysicalDatabaseBackupType.FULL_INCREMENTAL,
    description:
      'Full backups plus incremental ones that store only the changes since the previous backup. Smaller and faster.',
  },
  {
    label: 'Full + incremental + WAL',
    value: PhysicalDatabaseBackupType.FULL_INCREMENTAL_WAL_STREAM,
    description:
      'Adds continuous WAL streaming on top of full and incremental backups. Only this option enables point-in-time recovery (PITR), but it requires more space and is slower to restore.',
  },
];

// Drives the reused single-database editors in "wizard mode" (isSaveToApi=false) -
// same trick CreateDatabaseComponent uses for a brand-new database. Nothing here
// is ever persisted directly; it only exists to collect a config object.
const buildPlaceholderDatabase = (
  workspaceId: string,
  databaseType: DatabaseType,
  physicalBackupType: PhysicalDatabaseBackupType,
): Database =>
  initializeDatabaseTypeData({
    id: undefined as unknown as string,
    name: '',
    workspaceId,
    type: databaseType,
    notifiers: [],
    ...(databaseType === DatabaseType.POSTGRES_PHYSICAL
      ? {
          postgresqlPhysical: {
            backupType: physicalBackupType,
            sslMode: PostgresSslMode.Disable,
          } as PostgresqlPhysicalDatabase,
        }
      : {}),
  } as Database);

export const BulkBackupConfigModalComponent = ({
  selectedCount,
  open,
  onClose,
  onConfigured,
  workspaceId,
  instanceId,
  instanceType,
  selectedNames,
}: Props): JSX.Element => {
  const { message } = App.useApp();
  const isPostgres = instanceType === InstanceType.POSTGRES;

  const [step, setStep] = useState<WizardStep>(isPostgres ? 'backup-type' : 'backup-config');
  const [isPhysical, setIsPhysical] = useState(false);
  const [physicalBackupType, setPhysicalBackupType] = useState<PhysicalDatabaseBackupType>(
    PhysicalDatabaseBackupType.FULL,
  );
  const [logicalConfig, setLogicalConfig] = useState<LogicalBackupConfig>();
  const [physicalConfig, setPhysicalConfig] = useState<PhysicalBackupConfig>();
  const [isSaving, setIsSaving] = useState(false);

  const databaseType = isPostgres
    ? isPhysical
      ? DatabaseType.POSTGRES_PHYSICAL
      : DatabaseType.POSTGRES_LOGICAL
    : ENGINE_TO_LOGICAL_DATABASE_TYPE[instanceType];

  const placeholderDatabase = buildPlaceholderDatabase(workspaceId, databaseType, physicalBackupType);

  const finalizeBulkConfig = async (notifiers: Notifier[]) => {
    setIsSaving(true);

    try {
      const request: BulkConfigureBackupsRequest = {
        instanceId,
        databaseNames: selectedNames,
        backupType: isPhysical ? BulkBackupType.PHYSICAL : BulkBackupType.LOGICAL,
        logicalConfig: isPhysical ? undefined : logicalConfig,
        physicalConfig: isPhysical ? physicalConfig : undefined,
        notifierIds: notifiers.map((n) => n.id),
      };

      const response = await databaseInstancesApi.bulkConfigureBackups(request);

      const skippedNote =
        response.skipped.length > 0 ? `, ${response.skipped.length} already configured` : '';

      if (response.failed.length === 0) {
        message.success(`Backups configured for ${response.createdCount} databases${skippedNote}`);
      } else {
        message.warning(
          `Configured ${response.createdCount} databases${skippedNote}, ${response.failed.length} failed - first error: ${response.failed[0].name}: ${response.failed[0].error}`,
        );
      }

      onConfigured();
    } catch (error) {
      message.error((error as Error).message || 'Bulk configuration failed');
    }

    setIsSaving(false);
  };

  return (
    <Modal
      title={`Configure backup for ${selectedCount} selected database${selectedCount === 1 ? '' : 's'}`}
      open={open}
      onCancel={onClose}
      footer={null}
      maskClosable={false}
      destroyOnHidden
    >
      {step === 'backup-type' && (
        <div className="mt-3">
          <div className="mb-2 font-medium">Backup type</div>

          <Radio.Group
            value={isPhysical ? 'PHYSICAL' : 'LOGICAL'}
            onChange={(e) => setIsPhysical(e.target.value === 'PHYSICAL')}
            className="mb-4 w-full"
          >
            <div className="grid grid-cols-2 gap-3">
              <Radio.Button value="LOGICAL" className="h-auto! w-full py-2 text-center">
                Logical
              </Radio.Button>
              <Radio.Button value="PHYSICAL" className="h-auto! w-full py-2 text-center">
                Physical
              </Radio.Button>
            </div>
          </Radio.Group>

          {isPhysical && (
            <>
              <div className="mb-2 font-medium">Physical backup strategy</div>

              <Radio.Group
                value={physicalBackupType}
                onChange={(e) => {
                  setPhysicalBackupType(e.target.value);
                  // Retention rules differ per backup type (e.g. FULL_ONLY
                  // requires FULL_BACKUPS retention, no chains) - drop any
                  // config collected for the previous strategy so the editor
                  // recomputes defaults that actually match the new one,
                  // instead of carrying over a now-invalid retention.
                  setPhysicalConfig(undefined);
                }}
                className="mb-2 w-full"
              >
                <div className="flex flex-col gap-2">
                  {physicalBackupTypeOptions.map((option) => (
                    <Radio key={option.value} value={option.value}>
                      {option.label}
                      <Tooltip title={option.description}>
                        <InfoCircleOutlined className="ml-1 cursor-pointer" style={{ color: 'gray' }} />
                      </Tooltip>
                    </Radio>
                  ))}
                </div>
              </Radio.Group>

              <div className="mt-1 mb-4 max-w-[420px] rounded-md bg-amber-50 p-2 text-xs text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                Physical backups capture the entire server, not individual databases.
                {selectedCount > 1
                  ? ` You've selected ${selectedCount} databases - this creates ${selectedCount} independent full-server backup jobs against the same data, each with its own replication slot and storage footprint. For most cases, selecting just one database here (representing the whole server) is what you want.`
                  : ' This job backs up every database on the server, not just the one selected.'}
              </div>
            </>
          )}

          <div className="mt-5 flex">
            <Button danger ghost className="mr-1" onClick={onClose}>
              Cancel
            </Button>
            <Button type="primary" className="ml-auto" onClick={() => setStep('backup-config')}>
              Continue
            </Button>
          </div>
        </div>
      )}

      {step === 'backup-config' &&
        (isPhysical ? (
          <EditPhysicalBackupConfigComponent
            database={placeholderDatabase}
            initialConfig={physicalConfig}
            isShowBackButton={isPostgres}
            onBack={() => setStep('backup-type')}
            isShowCancelButton
            onCancel={onClose}
            saveButtonText="Continue"
            isSaveToApi={false}
            onSaved={(config) => {
              setPhysicalConfig(config);
              setStep('notifiers');
            }}
          />
        ) : (
          <EditLogicalBackupConfigComponent
            database={placeholderDatabase}
            isShowBackButton={isPostgres}
            onBack={() => setStep('backup-type')}
            isShowCancelButton
            onCancel={onClose}
            saveButtonText="Continue"
            isSaveToApi={false}
            onSaved={(config) => {
              setLogicalConfig(config);
              setStep('notifiers');
            }}
          />
        ))}

      {step === 'notifiers' && (
        <EditDatabaseNotifiersComponent
          database={placeholderDatabase}
          workspaceId={workspaceId}
          isShowBackButton
          onBack={() => setStep('backup-config')}
          isShowCancelButton
          onCancel={onClose}
          isShowSaveOnlyForUnsaved={false}
          saveButtonText={
            isSaving ? 'Applying...' : `Save & apply to ${selectedCount} database${selectedCount === 1 ? '' : 's'}`
          }
          isSaveToApi={false}
          onSaved={(database) => finalizeBulkConfig(database.notifiers)}
        />
      )}
    </Modal>
  );
};

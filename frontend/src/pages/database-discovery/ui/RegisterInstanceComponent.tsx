import { App, Button, Input, InputNumber, Select, Switch } from 'antd';
import { type JSX, useState } from 'react';

import { PostgresSslMode, getDatabaseLogoFromType } from '../../../entity/databases';
import { DatabaseType } from '../../../entity/databases';
import { databaseInstancesApi } from '../api/databaseInstancesApi';
import type { DatabaseInstance } from '../model/DatabaseInstance';
import { InstanceType } from '../model/InstanceType';

interface Props {
  workspaceId: string;
  // When set, the form edits this instance instead of creating a new one.
  // Password and client key come back blank from the API (server hides them),
  // so both fields are optional here and left untouched when submitted empty.
  existingInstance?: DatabaseInstance;
  onSaved: (instance: DatabaseInstance) => void;
  onCancel?: () => void;
}

const DEFAULT_PORTS: Record<InstanceType, number> = {
  [InstanceType.POSTGRES]: 5432,
  [InstanceType.MYSQL]: 3306,
  [InstanceType.MARIADB]: 3306,
  [InstanceType.MONGODB]: 27017,
};

// Instance engines map onto the existing per-database logos via the closest DatabaseType.
const engineOptions = [
  { type: InstanceType.POSTGRES, label: 'PostgreSQL', logoType: DatabaseType.POSTGRES_LOGICAL },
  { type: InstanceType.MYSQL, label: 'MySQL', logoType: DatabaseType.MYSQL },
  { type: InstanceType.MARIADB, label: 'MariaDB', logoType: DatabaseType.MARIADB },
  { type: InstanceType.MONGODB, label: 'MongoDB', logoType: DatabaseType.MONGODB },
];

const createInitialInstance = (workspaceId: string): DatabaseInstance => ({
  workspaceId,
  name: '',
  type: InstanceType.POSTGRES,
  host: '',
  port: DEFAULT_PORTS[InstanceType.POSTGRES],
  username: '',
  password: '',
  sslMode: PostgresSslMode.Disable,
  sslClientCert: '',
  sslClientKey: '',
  sslRootCert: '',
  isTlsEnabled: false,
  authDatabase: 'admin',
  isSrv: false,
});

// Password and client key are never returned by the API (HideSensitiveData
// clears them server-side), so an edit form always starts with those blank.
const createEditInitialInstance = (existing: DatabaseInstance): DatabaseInstance => ({
  ...existing,
  password: '',
  sslClientKey: '',
});

export const RegisterInstanceComponent = ({
  workspaceId,
  existingInstance,
  onSaved,
  onCancel,
}: Props): JSX.Element => {
  const { message } = App.useApp();
  const isEditMode = !!existingInstance?.id;
  const [instance, setInstance] = useState<DatabaseInstance>(() =>
    existingInstance
      ? createEditInitialInstance(existingInstance)
      : createInitialInstance(workspaceId),
  );
  const [isSaving, setIsSaving] = useState(false);

  const updateInstance = (patch: Partial<DatabaseInstance>) => {
    setInstance((prev) => ({ ...prev, ...patch }));
  };

  const handleTypeChange = (newType: InstanceType) => {
    updateInstance({ type: newType, port: DEFAULT_PORTS[newType] });
  };

  const saveInstance = async () => {
    setIsSaving(true);

    try {
      const payload = { ...instance, name: instance.name.trim(), host: instance.host.trim() };

      const saved = isEditMode
        ? await databaseInstancesApi.updateInstance(payload)
        : await databaseInstancesApi.registerInstance(payload);

      message.success(
        isEditMode
          ? `Instance "${saved.name}" updated`
          : `Instance "${saved.name}" registered - starting discovery`,
      );
      onSaved(saved);
    } catch (error) {
      message.error(
        (error as Error).message ||
          (isEditMode ? 'Failed to update instance' : 'Failed to register instance'),
      );
    }

    setIsSaving(false);
  };

  const isPostgres = instance.type === InstanceType.POSTGRES;
  const isMongodb = instance.type === InstanceType.MONGODB;
  const isMysqlFamily =
    instance.type === InstanceType.MYSQL || instance.type === InstanceType.MARIADB;

  const isPortRequired = !isMongodb || !instance.isSrv;

  // engineOptions covers every InstanceType, so this always resolves; the
  // fallback only satisfies the type checker.
  const currentEngineOption =
    engineOptions.find((option) => option.type === instance.type) ?? engineOptions[0];

  const showClientCertFields = isPostgres && instance.sslMode !== PostgresSslMode.Disable;
  const showRootCertField =
    isPostgres &&
    (instance.sslMode === PostgresSslMode.VerifyCa ||
      instance.sslMode === PostgresSslMode.VerifyFull);

  const isAllFieldsFilled =
    !!instance.name.trim() &&
    !!instance.host.trim() &&
    !!instance.username &&
    (isEditMode || !!instance.password) &&
    (!isPortRequired || !!instance.port);

  return (
    <div className="max-w-xl">
      <div className="mb-1 text-lg font-bold">
        {isEditMode ? 'Edit connection' : 'Connect a database server'}
      </div>

      <div className="mb-4 text-sm text-gray-500 dark:text-gray-400">
        {isEditMode
          ? 'Update the connection details. Leave password or client key blank to keep the current value.'
          : 'Register the server once - every database on it will be discovered automatically.'}
      </div>

      {isEditMode ? (
        <div className="mb-4 flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
          <img
            src={getDatabaseLogoFromType(currentEngineOption.logoType)}
            alt={currentEngineOption.label}
            className="h-5 w-5"
          />
          <span>{currentEngineOption.label}</span>
          <span className="text-xs">(engine type can&apos;t be changed after registration)</span>
        </div>
      ) : (
        <div className="mb-4 grid grid-cols-2 gap-3">
          {engineOptions.map((option) => {
            const isSelected = instance.type === option.type;

            return (
              <div
                key={option.type}
                onClick={() => handleTypeChange(option.type)}
                className={`flex h-24 cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border text-center text-xs transition hover:border-blue-400 ${
                  isSelected
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/40'
                    : 'border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800'
                }`}
              >
                <img
                  src={getDatabaseLogoFromType(option.logoType)}
                  alt={option.label}
                  className="h-7 w-7"
                />

                <span className="px-1 leading-tight">{option.label}</span>
              </div>
            );
          })}
        </div>
      )}

      <div className="mb-3 flex items-center">
        <div className="mr-3 w-28 shrink-0">Name</div>
        <Input
          value={instance.name}
          onChange={(e) => updateInstance({ name: e.target.value })}
          size="small"
          placeholder="Production cluster"
          className="grow"
        />
      </div>

      <div className="mb-3 flex items-center">
        <div className="mr-3 w-28 shrink-0">Host</div>
        <Input
          value={instance.host}
          onChange={(e) => updateInstance({ host: e.target.value })}
          size="small"
          placeholder="db.example.com"
          className="grow"
        />
      </div>

      {isMongodb && (
        <div className="mb-3 flex items-center">
          <div className="mr-3 w-28 shrink-0">SRV connection</div>
          <Switch
            checked={instance.isSrv}
            onChange={(isSrv) => updateInstance({ isSrv })}
            size="small"
          />
        </div>
      )}

      {isPortRequired && (
        <div className="mb-3 flex items-center">
          <div className="mr-3 w-28 shrink-0">Port</div>
          <InputNumber
            value={instance.port}
            onChange={(port) => updateInstance({ port: port ?? undefined })}
            size="small"
            min={1}
            max={65535}
            className="grow"
          />
        </div>
      )}

      <div className="mb-3 flex items-center">
        <div className="mr-3 w-28 shrink-0">Username</div>
        <Input
          value={instance.username}
          onChange={(e) => updateInstance({ username: e.target.value })}
          size="small"
          placeholder="admin"
          className="grow"
        />
      </div>

      <div className="mb-3 flex items-center">
        <div className="mr-3 w-28 shrink-0">Password</div>
        <Input.Password
          value={instance.password}
          onChange={(e) => updateInstance({ password: e.target.value })}
          size="small"
          className="grow"
          placeholder={isEditMode ? 'Leave blank to keep current password' : undefined}
        />
      </div>

      {isPostgres && (
        <div className="mb-3 flex items-center">
          <div className="mr-3 w-28 shrink-0">SSL mode</div>
          <Select
            value={instance.sslMode}
            onChange={(sslMode) => updateInstance({ sslMode })}
            size="small"
            className="grow"
            options={[
              { value: PostgresSslMode.Disable, label: 'disable' },
              { value: PostgresSslMode.Require, label: 'require' },
              { value: PostgresSslMode.VerifyCa, label: 'verify-ca' },
              { value: PostgresSslMode.VerifyFull, label: 'verify-full' },
            ]}
          />
        </div>
      )}

      {showRootCertField && (
        <div className="mb-3 flex items-start">
          <div className="mr-3 w-28 shrink-0 pt-1">Root CA cert</div>
          <Input.TextArea
            value={instance.sslRootCert}
            onChange={(e) => updateInstance({ sslRootCert: e.target.value })}
            placeholder="-----BEGIN CERTIFICATE-----"
            autoSize={{ minRows: 2, maxRows: 5 }}
            className="grow font-mono text-xs"
          />
        </div>
      )}

      {showClientCertFields && (
        <div className="mb-3 flex items-start">
          <div className="mr-3 w-28 shrink-0 pt-1">Client cert</div>
          <Input.TextArea
            value={instance.sslClientCert}
            onChange={(e) => updateInstance({ sslClientCert: e.target.value })}
            placeholder="-----BEGIN CERTIFICATE----- (optional, for mutual TLS)"
            autoSize={{ minRows: 2, maxRows: 5 }}
            className="grow font-mono text-xs"
          />
        </div>
      )}

      {showClientCertFields && (
        <div className="mb-3 flex items-start">
          <div className="mr-3 w-28 shrink-0 pt-1">Client key</div>
          <Input.TextArea
            value={instance.sslClientKey}
            onChange={(e) => updateInstance({ sslClientKey: e.target.value })}
            placeholder={
              isEditMode
                ? 'Leave blank to keep current key'
                : '-----BEGIN PRIVATE KEY----- (optional, for mutual TLS)'
            }
            autoSize={{ minRows: 2, maxRows: 5 }}
            className="grow font-mono text-xs"
          />
        </div>
      )}

      {isMysqlFamily && (
        <div className="mb-3 flex items-center">
          <div className="mr-3 w-28 shrink-0">Use TLS</div>
          <Switch
            checked={instance.isTlsEnabled}
            onChange={(isTlsEnabled) => updateInstance({ isTlsEnabled })}
            size="small"
          />
        </div>
      )}

      {isMongodb && (
        <div className="mb-3 flex items-center">
          <div className="mr-3 w-28 shrink-0">Auth database</div>
          <Input
            value={instance.authDatabase}
            onChange={(e) => updateInstance({ authDatabase: e.target.value })}
            size="small"
            placeholder="admin"
            className="grow"
          />
        </div>
      )}

      <div className="mt-5 flex">
        {onCancel && (
          <Button danger ghost className="mr-1" onClick={onCancel}>
            Cancel
          </Button>
        )}

        <Button
          type="primary"
          className={onCancel ? 'ml-1' : 'ml-auto'}
          onClick={saveInstance}
          loading={isSaving}
          disabled={!isAllFieldsFilled}
        >
          {isEditMode ? 'Save changes' : 'Connect and discover'}
        </Button>
      </div>
    </div>
  );
};

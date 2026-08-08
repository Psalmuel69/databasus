import { App, Form, Modal, Select, Switch, TimePicker } from 'antd';
import dayjs from 'dayjs';
import { type JSX, useEffect, useState } from 'react';

import {
  LogicalBackupNotificationType,
  LogicalRetentionPolicyType,
} from '../../../entity/backups/logical';
import { BackupEncryption } from '../../../entity/backups/shared';
import { Period } from '../../../entity/databases';
import { IntervalType } from '../../../entity/intervals';
import { type Storage, storageApi } from '../../../entity/storages';
import { databaseInstancesApi } from '../api/databaseInstancesApi';

interface Props {
  selectedCount: number;
  open: boolean;
  onClose: () => void;
  onConfigured: () => void;

  // Real mode - when provided, Save calls the bulk-configure endpoint.
  // Without them the modal stays a visual mock (standalone preview).
  workspaceId?: string;
  instanceId?: string;
  selectedNames?: string[];
}

interface BulkBackupFormValues {
  intervalType: IntervalType.HOURLY | IntervalType.DAILY | IntervalType.WEEKLY;
  timeOfDay: dayjs.Dayjs;
  retentionTimePeriod: Period;
  isEncryptionEnabled: boolean;
  storageId?: string;
}

const RETENTION_OPTIONS = [
  { value: Period.WEEK, label: '1 week' },
  { value: Period.MONTH, label: '1 month' },
  { value: Period.THREE_MONTH, label: '3 months' },
  { value: Period.SIX_MONTH, label: '6 months' },
  { value: Period.YEAR, label: '1 year' },
  { value: Period.FOREVER, label: 'Forever' },
];

export const BulkBackupConfigModalComponent = ({
  selectedCount,
  open,
  onClose,
  onConfigured,
  workspaceId,
  instanceId,
  selectedNames,
}: Props): JSX.Element => {
  const { message } = App.useApp();
  const [form] = Form.useForm<BulkBackupFormValues>();
  const [isSaving, setIsSaving] = useState(false);
  const [storages, setStorages] = useState<Storage[]>([]);

  const isRealMode = !!instanceId && !!selectedNames && selectedNames.length > 0;

  const loadStorages = async () => {
    if (!workspaceId) return;

    try {
      setStorages(await storageApi.getStorages(workspaceId));
    } catch (error) {
      message.error((error as Error).message || 'Failed to load storages');
    }
  };

  const submitBulkConfig = async (values: BulkBackupFormValues) => {
    setIsSaving(true);

    if (!isRealMode) {
      // Mock save for the standalone preview.
      await new Promise((resolve) => setTimeout(resolve, 600));
      message.success(`Backup jobs created for ${selectedCount} databases`);
      setIsSaving(false);
      onConfigured();

      return;
    }

    try {
      const storage = storages.find((s) => s.id === values.storageId);

      const response = await databaseInstancesApi.bulkConfigureBackups({
        instanceId: instanceId,
        databaseNames: selectedNames,
        backupConfig: {
          isBackupsEnabled: true,
          backupInterval: {
            type: values.intervalType,
            timeOfDay: values.timeOfDay.format('HH:mm'),
          },
          storage,
          retentionPolicyType: LogicalRetentionPolicyType.TimePeriod,
          retentionTimePeriod: values.retentionTimePeriod,
          retentionCount: 100,
          retentionGfsHours: 24,
          retentionGfsDays: 7,
          retentionGfsWeeks: 4,
          retentionGfsMonths: 12,
          retentionGfsYears: 3,
          sendNotificationsOn: [LogicalBackupNotificationType.BackupFailed],
          isRetryIfFailed: true,
          maxFailedTriesCount: 3,
          encryption: values.isEncryptionEnabled
            ? BackupEncryption.ENCRYPTED
            : BackupEncryption.NONE,
        },
      });

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

  useEffect(() => {
    if (open) {
      loadStorages();
    }
  }, [open, workspaceId]);

  return (
    <Modal
      title={`Configure backup for ${selectedCount} selected databases`}
      open={open}
      onCancel={onClose}
      onOk={() => form.submit()}
      okText="Save"
      okButtonProps={{ loading: isSaving }}
      maskClosable={false}
      destroyOnHidden
    >
      <Form<BulkBackupFormValues>
        form={form}
        layout="vertical"
        className="mt-4"
        initialValues={{
          intervalType: IntervalType.DAILY,
          timeOfDay: dayjs('02:00', 'HH:mm'),
          retentionTimePeriod: Period.THREE_MONTH,
          isEncryptionEnabled: true,
        }}
        onFinish={submitBulkConfig}
      >
        <div className="flex gap-3">
          <Form.Item
            name="intervalType"
            label="Frequency"
            className="flex-1"
            rules={[{ required: true }]}
          >
            <Select
              options={[
                { value: IntervalType.HOURLY, label: 'Hourly' },
                { value: IntervalType.DAILY, label: 'Daily' },
                { value: IntervalType.WEEKLY, label: 'Weekly' },
              ]}
            />
          </Form.Item>

          <Form.Item name="timeOfDay" label="Time" className="flex-1" rules={[{ required: true }]}>
            <TimePicker className="w-full" format="HH:mm" />
          </Form.Item>
        </div>

        <Form.Item name="retentionTimePeriod" label="Retention" rules={[{ required: true }]}>
          <Select options={RETENTION_OPTIONS} />
        </Form.Item>

        <Form.Item
          name="storageId"
          label="Storage"
          rules={[{ required: isRealMode, message: 'Storage is required' }]}
        >
          <Select
            placeholder={
              storages.length > 0 ? 'Select storage' : 'No storages available in this workspace'
            }
            options={storages.map((storage) => ({ value: storage.id, label: storage.name }))}
          />
        </Form.Item>

        <Form.Item name="isEncryptionEnabled" label="Encryption" valuePropName="checked">
          <Switch />
        </Form.Item>
      </Form>
    </Modal>
  );
};

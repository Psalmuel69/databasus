import { DeleteOutlined, EditOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { App, Button, Input, Popconfirm, Select, Table } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import { type JSX, useMemo, useState } from 'react';

import { getUserTimeFormat } from '../../../shared/time';
import type { DiscoveredInstance } from '../model/DiscoveredInstance';
import { DiscoveryStatus } from '../model/DiscoveryStatus';
import type { FleetDatabase } from '../model/FleetDatabase';
import { generateMockFleetDatabases } from '../model/generateMockFleetDatabases';
import type { InstanceType } from '../model/InstanceType';
import { BulkBackupConfigModalComponent } from './BulkBackupConfigModalComponent';
import { BulkSelectionMenuComponent } from './BulkSelectionMenuComponent';
import { DatabaseDiscoveryStatusTag } from './DatabaseDiscoveryStatusTag';

const PAGE_SIZE = 50;
const NEWLY_DISCOVERED_COUNT = 3;

const MOCK_INSTANCE: DiscoveredInstance = {
  host: 'prod-pg-cluster-01.internal',
  port: 5432,
  engineLabel: 'PostgreSQL',
  engineIconSrc: '/icons/databases/postgresql.svg',
  lastDiscoveredAt: new Date(Date.now() - 42 * 60_000),
};

const formatSizeMb = (sizeMb?: number): string => {
  if (sizeMb === undefined) {
    return '-';
  }

  if (sizeMb >= 1024) {
    return `${(sizeMb / 1024).toFixed(1)} GB`;
  }

  return `${sizeMb} MB`;
};

const buildNewlyDiscoveredDatabases = (existingCount: number): FleetDatabase[] =>
  Array.from({ length: NEWLY_DISCOVERED_COUNT }, (_, index) => ({
    id: `customer_${existingCount + index + 1}`,
    name: `customer_${existingCount + index + 1}`,
    status: DiscoveryStatus.DISCOVERED,
  }));

interface Props {
  instance?: DiscoveredInstance;
  initialDatabases?: FleetDatabase[];
  onRefreshDiscovery?: () => Promise<FleetDatabase[]>;
  workspaceId?: string;
  instanceId?: string;
  instanceType?: InstanceType;
  onEditInstance?: () => void;
  onDeleteInstance?: () => void;
}

export const DatabaseDiscoveryPageComponent = ({
  instance,
  initialDatabases,
  onRefreshDiscovery,
  workspaceId,
  instanceId,
  instanceType,
  onEditInstance,
  onDeleteInstance,
}: Props): JSX.Element => {
  const { message } = App.useApp();
  const [databases, setDatabases] = useState<FleetDatabase[]>(
    () => initialDatabases ?? generateMockFleetDatabases(),
  );
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<DiscoveryStatus[]>([]);
  const [currentPage, setCurrentPage] = useState(1);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isBulkConfigOpen, setIsBulkConfigOpen] = useState(false);

  const refreshDiscovery = async () => {
    setIsRefreshing(true);

    if (onRefreshDiscovery) {
      try {
        const refreshed = await onRefreshDiscovery();
        setDatabases(refreshed);
        message.success(`Refresh complete - ${refreshed.length} databases discovered`);
      } catch (error) {
        message.error((error as Error).message || 'Discovery failed');
      }

      setIsRefreshing(false);

      return;
    }

    // Mock rescan - a real refresh calls the discovery endpoint and diffs the result
    // against stored databases without touching existing backup configurations.
    await new Promise((resolve) => setTimeout(resolve, 900));

    const newDatabases = buildNewlyDiscoveredDatabases(databases.length);
    setDatabases([...databases, ...newDatabases]);
    setIsRefreshing(false);
    message.success(`Refresh complete - ${newDatabases.length} new databases discovered`);
  };

  const selectIds = (ids: string[]) => {
    setSelectedIds(new Set(ids));
  };

  const clearSelection = () => {
    setSelectedIds(new Set());
  };

  const closeBulkConfig = () => {
    setIsBulkConfigOpen(false);
  };

  const confirmBulkConfig = () => {
    setIsBulkConfigOpen(false);
    clearSelection();

    if (onRefreshDiscovery) {
      refreshDiscovery();
    }
  };

  const filteredDatabases = useMemo(
    () =>
      databases.filter((database) => {
        const matchesSearch = database.name.toLowerCase().includes(searchQuery.toLowerCase());
        const matchesStatus = statusFilter.length === 0 || statusFilter.includes(database.status);

        return matchesSearch && matchesStatus;
      }),
    [databases, searchQuery, statusFilter],
  );

  const visibleDatabases = useMemo(
    () => filteredDatabases.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE),
    [filteredDatabases, currentPage],
  );

  const columns: ColumnsType<FleetDatabase> = [
    {
      title: 'Database name',
      dataIndex: 'name',
      key: 'name',
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (name: string) => <span className="font-medium">{name}</span>,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      width: 160,
      sorter: (a, b) => a.status.localeCompare(b.status),
      render: (status: DiscoveryStatus) => <DatabaseDiscoveryStatusTag status={status} />,
    },
    {
      title: 'Size',
      dataIndex: 'sizeMb',
      key: 'sizeMb',
      width: 120,
      sorter: (a, b) => (a.sizeMb ?? 0) - (b.sizeMb ?? 0),
      render: (sizeMb?: number) => formatSizeMb(sizeMb),
    },
    {
      title: 'Owner',
      dataIndex: 'owner',
      key: 'owner',
      width: 160,
      render: (owner?: string) => owner ?? '-',
    },
    {
      title: 'Last backup',
      dataIndex: 'lastBackupTime',
      key: 'lastBackupTime',
      width: 180,
      sorter: (a, b) => (a.lastBackupTime?.getTime() ?? 0) - (b.lastBackupTime?.getTime() ?? 0),
      render: (lastBackupTime?: Date) =>
        lastBackupTime ? (
          <span title={dayjs(lastBackupTime).format(getUserTimeFormat().format)}>
            {dayjs(lastBackupTime).fromNow()}
          </span>
        ) : (
          <span className="text-gray-400 dark:text-gray-500">Never</span>
        ),
    },
    {
      title: 'Recovery model',
      dataIndex: 'recoveryModel',
      key: 'recoveryModel',
      width: 150,
      render: (recoveryModel?: string) => recoveryModel ?? '-',
    },
  ];

  const shownInstance = instance ?? MOCK_INSTANCE;

  return (
    <div className="flex h-full flex-col">
      <div className="mb-4 flex items-center justify-between rounded-lg bg-white p-4 shadow dark:bg-gray-800">
        <div className="flex items-center">
          <img
            src={shownInstance.engineIconSrc}
            alt={shownInstance.engineLabel}
            className="mr-3 h-8 w-8"
          />

          <div>
            <div className="font-bold">
              {shownInstance.host}:{shownInstance.port}
            </div>
            <div className="text-sm text-gray-500 dark:text-gray-400">
              {shownInstance.engineLabel} - {databases.length.toLocaleString()} databases discovered
              - last scanned {dayjs(shownInstance.lastDiscoveredAt).fromNow()}
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {onEditInstance && (
            <Button icon={<EditOutlined />} onClick={onEditInstance}>
              Edit connection
            </Button>
          )}

          {onDeleteInstance && (
            <Popconfirm
              title="Remove this instance?"
              description="This only removes the registered connection - already configured backups are not affected."
              onConfirm={onDeleteInstance}
              okText="Remove"
              okButtonProps={{ danger: true }}
            >
              <Button danger icon={<DeleteOutlined />}>
                Remove
              </Button>
            </Popconfirm>
          )}

          <Button icon={<ReloadOutlined />} loading={isRefreshing} onClick={refreshDiscovery}>
            Refresh discovery
          </Button>
        </div>
      </div>

      <div className="mb-3 flex flex-wrap items-center gap-2">
        <Input
          placeholder="Search databases"
          prefix={<SearchOutlined className="text-gray-400" />}
          value={searchQuery}
          onChange={(e) => {
            setSearchQuery(e.target.value);
            setCurrentPage(1);
          }}
          className="w-64"
          allowClear
        />

        <Select
          mode="multiple"
          placeholder="Filter by status"
          value={statusFilter}
          onChange={(values) => {
            setStatusFilter(values);
            setCurrentPage(1);
          }}
          className="w-64"
          options={[
            { value: DiscoveryStatus.CONFIGURED, label: 'Configured' },
            { value: DiscoveryStatus.DISCOVERED, label: 'Discovered' },
            { value: DiscoveryStatus.OFFLINE, label: 'Offline' },
            { value: DiscoveryStatus.INACCESSIBLE, label: 'Inaccessible' },
          ]}
        />

        <BulkSelectionMenuComponent
          filteredDatabases={filteredDatabases}
          visibleDatabases={visibleDatabases}
          onSelectIds={selectIds}
          onClearSelection={clearSelection}
        />

        <div className="ml-auto flex items-center gap-3">
          {selectedIds.size > 0 && (
            <span className="text-sm text-gray-500 dark:text-gray-400">
              {selectedIds.size.toLocaleString()} selected
            </span>
          )}

          <Button
            type="primary"
            disabled={selectedIds.size === 0 || !workspaceId || !instanceId || !instanceType}
            onClick={() => setIsBulkConfigOpen(true)}
          >
            Configure backup
          </Button>
        </div>
      </div>

      <Table<FleetDatabase>
        columns={columns}
        dataSource={filteredDatabases}
        rowKey="id"
        size="middle"
        rowSelection={{
          selectedRowKeys: Array.from(selectedIds),
          onChange: (keys) => setSelectedIds(new Set(keys as string[])),
        }}
        pagination={{
          current: currentPage,
          pageSize: PAGE_SIZE,
          total: filteredDatabases.length,
          showSizeChanger: false,
          showTotal: (total) => `${total.toLocaleString()} databases`,
          onChange: setCurrentPage,
        }}
        scroll={{ y: 'calc(100vh - 320px)' }}
      />

      {isBulkConfigOpen && workspaceId && instanceId && instanceType && (
        <BulkBackupConfigModalComponent
          selectedCount={selectedIds.size}
          open={isBulkConfigOpen}
          onClose={closeBulkConfig}
          onConfigured={confirmBulkConfig}
          workspaceId={workspaceId}
          instanceId={instanceId}
          instanceType={instanceType}
          selectedNames={Array.from(selectedIds)}
        />
      )}
    </div>
  );
};

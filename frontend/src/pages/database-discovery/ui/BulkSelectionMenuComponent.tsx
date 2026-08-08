import { DownOutlined } from '@ant-design/icons';
import { Button, Divider, Dropdown, Input, InputNumber, Space } from 'antd';
import { type JSX, useState } from 'react';

import type { FleetDatabase } from '../model/FleetDatabase';
import { matchesNamePattern } from '../model/matchesNamePattern';

interface Props {
  filteredDatabases: FleetDatabase[];
  visibleDatabases: FleetDatabase[];
  onSelectIds: (ids: string[]) => void;
  onClearSelection: () => void;
}

export const BulkSelectionMenuComponent = ({
  filteredDatabases,
  visibleDatabases,
  onSelectIds,
  onClearSelection,
}: Props): JSX.Element => {
  const [namePattern, setNamePattern] = useState('');
  const [minSizeGb, setMinSizeGb] = useState<number | null>(null);

  const selectByNamePattern = () => {
    const matchingIds = filteredDatabases
      .filter((database) => matchesNamePattern(database.name, namePattern))
      .map((database) => database.id);

    onSelectIds(matchingIds);
  };

  const selectBySizeThreshold = () => {
    if (minSizeGb === null) {
      return;
    }

    const matchingIds = filteredDatabases
      .filter((database) => (database.sizeMb ?? 0) >= minSizeGb * 1024)
      .map((database) => database.id);

    onSelectIds(matchingIds);
  };

  const dropdownContent = (
    <div className="w-80 rounded-lg bg-white p-3 shadow-lg dark:bg-gray-800">
      <Space direction="vertical" className="w-full" size="small">
        <Button
          type="text"
          className="!justify-start"
          block
          onClick={() => onSelectIds(filteredDatabases.map((database) => database.id))}
        >
          Select all {filteredDatabases.length} matching databases
        </Button>

        <Button
          type="text"
          className="!justify-start"
          block
          onClick={() => onSelectIds(visibleDatabases.map((database) => database.id))}
        >
          Select {visibleDatabases.length} visible on this page
        </Button>

        <Button type="text" className="!justify-start" block onClick={onClearSelection}>
          Select none
        </Button>
      </Space>

      <Divider className="my-3" />

      <div className="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
        Select by filter
      </div>

      <Space direction="vertical" className="w-full" size="small">
        <Space.Compact className="w-full">
          <Input
            placeholder="Name matches, e.g. customer_* or finance*"
            value={namePattern}
            onChange={(e) => setNamePattern(e.target.value)}
            onPressEnter={selectByNamePattern}
          />
          <Button onClick={selectByNamePattern} disabled={!namePattern.trim()}>
            Apply
          </Button>
        </Space.Compact>

        <Space.Compact className="w-full">
          <InputNumber
            className="!w-full"
            placeholder="Minimum size (GB)"
            min={0}
            value={minSizeGb}
            onChange={setMinSizeGb}
          />
          <Button onClick={selectBySizeThreshold} disabled={minSizeGb === null}>
            Apply
          </Button>
        </Space.Compact>
      </Space>
    </div>
  );

  return (
    <Dropdown popupRender={() => dropdownContent} trigger={['click']}>
      <Button>
        Select <DownOutlined />
      </Button>
    </Dropdown>
  );
};

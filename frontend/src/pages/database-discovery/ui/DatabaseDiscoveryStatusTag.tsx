import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  MinusCircleOutlined,
  StopOutlined,
} from '@ant-design/icons';
import type { JSX } from 'react';

import { DiscoveryStatus } from '../model/DiscoveryStatus';

interface Props {
  status: DiscoveryStatus;
}

export const DatabaseDiscoveryStatusTag = ({ status }: Props): JSX.Element => {
  if (status === DiscoveryStatus.CONFIGURED) {
    return (
      <div className="flex items-center text-green-600 dark:text-green-400">
        <CheckCircleOutlined className="mr-2" />
        <span>Configured</span>
      </div>
    );
  }

  if (status === DiscoveryStatus.DISCOVERED) {
    return (
      <div className="flex items-center text-amber-500 dark:text-amber-400">
        <ExclamationCircleOutlined className="mr-2" />
        <span>Discovered</span>
      </div>
    );
  }

  if (status === DiscoveryStatus.OFFLINE) {
    return (
      <div className="flex items-center text-red-600 dark:text-red-400">
        <MinusCircleOutlined className="mr-2" />
        <span>Offline</span>
      </div>
    );
  }

  return (
    <div className="flex items-center text-gray-500 dark:text-gray-400">
      <StopOutlined className="mr-2" />
      <span>Inaccessible</span>
    </div>
  );
};

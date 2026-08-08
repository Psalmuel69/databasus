import type { PostgresSslMode } from '../../../entity/databases';
import type { InstanceType } from './InstanceType';

export interface DatabaseInstance {
  id?: string;
  workspaceId: string;
  name: string;
  type: InstanceType;

  host: string;
  port?: number;
  username: string;
  password?: string;

  sslMode?: PostgresSslMode;
  sslClientCert?: string;
  sslClientKey?: string;
  sslRootCert?: string;

  isTlsEnabled?: boolean;

  authDatabase?: string;
  isSrv?: boolean;

  lastDiscoveredAt?: string;
  createdAt?: string;
}

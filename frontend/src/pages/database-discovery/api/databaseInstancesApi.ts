import { getApplicationServer } from '../../../constants';
import RequestOptions from '../../../shared/api/RequestOptions';
import { apiHelper } from '../../../shared/api/apiHelper';
import type { BulkConfigureBackupsRequest } from '../model/BulkConfigureBackupsRequest';
import type { BulkConfigureBackupsResponse } from '../model/BulkConfigureBackupsResponse';
import type { DatabaseInstance } from '../model/DatabaseInstance';
import type { DiscoverDatabasesResponse } from '../model/DiscoverDatabasesResponse';

export const databaseInstancesApi = {
  async registerInstance(instance: DatabaseInstance) {
    const requestOptions = new RequestOptions();
    requestOptions.setBody(JSON.stringify(instance));

    return apiHelper.fetchPostJson<DatabaseInstance>(
      `${getApplicationServer()}/api/v1/database-instances/create`,
      requestOptions,
    );
  },

  async updateInstance(instance: DatabaseInstance) {
    const requestOptions = new RequestOptions();
    requestOptions.setBody(JSON.stringify(instance));

    return apiHelper.fetchPostJson<DatabaseInstance>(
      `${getApplicationServer()}/api/v1/database-instances/update`,
      requestOptions,
    );
  },

  async getInstances(workspaceId: string) {
    return apiHelper.fetchGetJson<DatabaseInstance[]>(
      `${getApplicationServer()}/api/v1/database-instances?workspaceId=${workspaceId}`,
      undefined,
      true,
    );
  },

  async getInstance(instanceId: string) {
    return apiHelper.fetchGetJson<DatabaseInstance>(
      `${getApplicationServer()}/api/v1/database-instances/${instanceId}`,
      undefined,
      true,
    );
  },

  async deleteInstance(instanceId: string) {
    return apiHelper.fetchDeleteJson<void>(
      `${getApplicationServer()}/api/v1/database-instances/${instanceId}`,
    );
  },

  async discoverDatabases(instanceId: string) {
    return apiHelper.fetchPostJson<DiscoverDatabasesResponse>(
      `${getApplicationServer()}/api/v1/database-instances/${instanceId}/discover`,
      new RequestOptions(),
    );
  },

  async bulkConfigureBackups(request: BulkConfigureBackupsRequest) {
    const requestOptions = new RequestOptions();
    requestOptions.setBody(JSON.stringify(request));

    return apiHelper.fetchPostJson<BulkConfigureBackupsResponse>(
      `${getApplicationServer()}/api/v1/database-instances/bulk-configure-backups`,
      requestOptions,
    );
  },
};

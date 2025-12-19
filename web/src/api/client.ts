import axios from 'axios';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

export interface Operation {
  id: string;
  block_num: number;
  trx_id: string;
  account: string;
  op_type: string;
  op_data: Record<string, any>;
  timestamp: string;
  created_at: string;
}

export interface OperationResponse {
  operations: Operation[];
  total: number;
  page: number;
  page_size: number;
  has_more: boolean;
}

export const api = {
  getAccounts: async (): Promise<string[]> => {
    const response = await apiClient.get<{ accounts: string[] }>('/accounts');
    return response.data.accounts;
  },

  getOperations: async (
    account: string,
    type?: string,
    page: number = 1,
    pageSize: number = 20
  ): Promise<OperationResponse> => {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    });
    if (type) {
      params.append('type', type);
    }
    const response = await apiClient.get<OperationResponse>(
      `/accounts/${account}/operations?${params.toString()}`
    );
    return response.data;
  },

  getTransfers: async (
    account: string,
    page: number = 1,
    pageSize: number = 20
  ): Promise<OperationResponse> => {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    });
    const response = await apiClient.get<OperationResponse>(
      `/accounts/${account}/transfers?${params.toString()}`
    );
    return response.data;
  },

  getUpdates: async (
    account: string,
    page: number = 1,
    pageSize: number = 20
  ): Promise<OperationResponse> => {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    });
    const response = await apiClient.get<OperationResponse>(
      `/accounts/${account}/updates?${params.toString()}`
    );
    return response.data;
  },
};


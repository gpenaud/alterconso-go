import api from "./client";

export interface AccountUser {
  id: number;
  firstName: string;
  lastName: string;
  email: string;
  phone?: string;
  address1?: string;
  zipCode?: string;
  city?: string;
}

export interface AccountOrderRow {
  productName: string;
  smartQty: string;
  total: number;
  paid: boolean;
  date: string;
}

export interface AccountSubRow {
  catalogName: string;
  startDate: string;
  endDate?: string;
}

export interface AccountResponse {
  user: AccountUser;
  recentOrders: AccountOrderRow[];
  subscriptions: AccountSubRow[];
  membershipRenewalPeriod?: string;
}

export function fetchAccount() {
  return api.get<AccountResponse>("/account").then((r) => r.data);
}

export function updateAccount(payload: Partial<AccountUser>) {
  return api.put("/users/me", payload).then((r) => r.data);
}

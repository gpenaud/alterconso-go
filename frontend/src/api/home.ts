import api from "./client";

export interface UserOrderView {
  productName: string;
  smartQty: string;
  unitPrice: number;
  subTotal: number;
  fees: number;
  total: number;
}

export interface ProductImageView {
  url: string;
  name: string;
}

export interface MultiDistribView {
  id: number;
  place: string;
  placeAddress: string;
  dayOfWeek: string;
  day: string;
  month: string;
  startHour: string;
  endHour: string;
  dayLabelFull: string;
  active: boolean;
  past: boolean;
  canOrder: boolean;
  orderNotYetOpen: boolean;
  orderStartDate?: string;
  orderEndDate?: string;
  distributions: boolean;
  userOrders?: UserOrderView[];
  userOrderTotal: number;
  productImages?: ProductImageView[];
  volunteerNeeded: number;
  volunteerRoles?: string[];
}

export interface HomeResponse {
  groupId: number;
  groupName: string;
  groupTxtHome?: string;
  offset: number;
  periodLabel: string;
  multiDistribs: MultiDistribView[];
}

export function fetchHome(offset = 0) {
  return api
    .get<HomeResponse>("/home", { params: { offset } })
    .then((r) => r.data);
}

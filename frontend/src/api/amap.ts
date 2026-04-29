import api from "./client";

export interface AmapPerson {
  firstName: string;
  lastName: string;
  email?: string;
  phone?: string;
}

export interface AmapProductImage {
  url: string;
  name: string;
}

export interface AmapCatalog {
  id: number;
  name: string;
  productImages: AmapProductImage[];
  coordinator?: AmapPerson;
}

export interface AmapVendor {
  id: number;
  name: string;
  city?: string;
  zipCode?: string;
  catalogs: AmapCatalog[];
}

export interface AmapResponse {
  group: { id: number; name: string };
  contact?: AmapPerson;
  vendors: AmapVendor[];
  isGroupManager: boolean;
}

export function fetchAmap() {
  return api.get<AmapResponse>("/amap").then((r) => r.data);
}

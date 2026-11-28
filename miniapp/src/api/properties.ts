import { ApiError, request } from "./client";

export interface Property {
  editRevision?: number;
  id: string;
  title: string;
  marketingPhrase?: string;
  community: string;
  location: string;
  areaLabel?: string;
  coverFocusX?: number;
  coverFocusY?: number;
  doorplateLabel?: string;
  displayTags?: string[];
  buildingNo?: string;
  unitNo?: string;
  roomNo?: string;
  roomSequence?: number;
  isMaterial?: boolean;
  materialCategory?: string;
  convertedPropertyId?: string;
  latitude?: number;
  longitude?: number;
  layout: string;
  actualLayout?: string;
  wholeLayout?: string;
  rentMonthly: number;
  feeText: string;
  rentType?: string;
  listingSource?: string;
  remainingLeaseMonths?: number;
  transferDeposit?: number;
  availableDate?: string;
  depositPayment?: string;
  utilitiesText?: string;
  areaSqm?: number;
  actualAreaSqm?: number;
  wholeAreaSqm?: number;
  floorInfo?: string;
  orientation?: string;
  elevator?: string;
  metroMinutes?: number;
  verificationDate?: string;
  rulesText?: string;
  description: string;
  status: string;
  ownerId?: string;
  media?: PropertyMedia[];
}
export interface PropertyMedia {
  id: string;
  propertyId?: string;
  type: string;
  url: string;
  sort?: number;
}
export interface FeaturedChannel {
  id?: string;
  icon: string;
  title: string;
  subtitle: string;
  community?: string;
  keyword: string;
  latitude?: number;
  longitude?: number;
  radiusKm?: number;
  showInAreaEntry?: boolean;
}
export const listProperties = () => request<Property[]>("/v1/properties");
export const getPropertyDetail = async (id: string) => {
  try {
    return await request<Property>(`/v1/properties/${encodeURIComponent(id)}`);
  } catch (error) {
    // Older API deployments do not expose the detail route yet.
    if (!(error instanceof ApiError) || error.statusCode !== 404 || error.code === 404) throw error;
    const item = (await listProperties()).find(property => property.id === id);
    if (!item) throw error;
    return item;
  }
};
export const listMapProperties = () => request<Property[]>("/v1/properties?view=map");
export const createProperty = (property: Partial<Property> & { createKey?: string }) =>
  request<Property>("/v1/properties", { method: "POST", data: property });
export const updateProperty = (property: Partial<Property> & { editBatch?: string; mediaIds?: string[] }) =>
  request<Property>("/v1/properties/update", {
    method: "PATCH",
    data: property,
  });
export const listMaterialLibrary = () => request<Property[]>('/v1/admin/material-library');
export const createMaterial = (property: Partial<Property> & { createKey?: string }) =>
  request<Property>('/v1/admin/material-library', { method: 'POST', data: property });
export const deleteMaterial = (propertyId: string) =>
  request(`/v1/admin/material-library/${encodeURIComponent(propertyId)}`, { method: 'DELETE' });
export const convertMaterial = (propertyId: string) =>
  request<Property>(`/v1/admin/material-library/${encodeURIComponent(propertyId)}/convert`, { method: 'PATCH' });
export const getMaterialCategories = () => request<string[]>('/v1/admin/material-library/categories');
export const saveMaterialCategories = (change: { add?: string; remove?: string }) =>
  request<string[]>('/v1/admin/material-library/categories', { method: 'PUT', data: change });
export const renameMaterialCategory = (from: string, to: string) =>
  request<{ from: string; to: string }>('/v1/admin/material-library/categories/rename', { method: 'PATCH', data: { from, to } });
export const deleteProperty = (propertyId: string) =>
  request(`/v1/properties/${encodeURIComponent(propertyId)}`, { method: "DELETE" });
export const replacePropertyMedia = (propertyId: string, mediaIds: string[]) =>
  request("/v1/properties/media", {
    method: "PATCH",
    data: { propertyId, mediaIds },
  });
export interface LocationSearchItem {
  id: string;
  title: string;
  address: string;
  district: string;
  latitude: number;
  longitude: number;
  distance?: number;
}
export const searchLocations = (
  keyword: string,
  latitude: number,
  longitude: number,
) =>
  request<LocationSearchItem[]>(
    `/v1/admin/location/search?keyword=${encodeURIComponent(keyword)}&latitude=${latitude}&longitude=${longitude}`,
  );
export interface BuildingRule {
  id?: string;
  community: string;
  buildingNo: string;
  unitCount: number;
  floorStart: number;
  floorCount: number;
  householdsPerFloor: number;
}
export const listBuildingRules = (community: string) =>
  request<BuildingRule[]>(`/v1/admin/building-rules?community=${encodeURIComponent(community)}`);
export const saveBuildingRule = (rule: BuildingRule) =>
  request<BuildingRule>("/v1/admin/building-rules", { method: "PUT", data: rule });
export interface PropertyLead {
  id: string;
  sourceName: string;
  sourcePhone: string;
  community: string;
  location: string;
  layout: string;
  expectedRentMonthly: number;
  note: string;
  status: string;
	followUpNote?: string;
}
export const createPropertyLead = (lead: Partial<PropertyLead>) =>
  request<PropertyLead>("/v1/property-leads", { method: "POST", data: lead });
export interface RentalRequest {
  id?: string;
  requestType: "general" | "appointment" | "area_subscription";
  propertyId?: string;
	propertyTitle?: string;
	propertyAvailable?: boolean;
	contactName?: string;
  phone: string;
	preferredTime?: string;
  location: string;
  layout: string;
  budgetMin: number;
  budgetMax: number;
  moveInDate: string;
  note: string;
  status?: string;
	followUpNote?: string;
  createdAt?: string;
}
export interface OperationLog { id: string; targetType: string; targetId: string; action: string; detail: string; createdAt: string }
export interface OperationLogPage { items: OperationLog[]; total: number; page: number; pageSize: number; pageCount: number }
export const queryOperationLogs = (filters: { category: string; startDate?: string; endDate?: string; page: number; pageSize?: number }) => {
  const params = [
    `category=${encodeURIComponent(filters.category)}`,
    `page=${filters.page}`,
    `pageSize=${filters.pageSize || 20}`,
  ];
  if (filters.startDate) params.push(`startDate=${encodeURIComponent(filters.startDate)}`);
  if (filters.endDate) params.push(`endDate=${encodeURIComponent(filters.endDate)}`);
  return request<OperationLogPage>(`/v1/admin/operation-logs/query?${params.join("&")}`);
};
export interface FavoriteContact { userId: string; nickname: string; avatarUrl: string; phone: string; propertyId: string; propertyName: string; createdAt: string }
export const createRentalRequest = (item: RentalRequest) =>
  request<RentalRequest>("/v1/rental-requests", { method: "POST", data: item });
export const listMyRentalRequests = () =>
  request<RentalRequest[]>("/v1/rental-requests/mine");
export interface ContactEvent {
  id?: string;
  userId?: string;
  propertyId: string;
  type: "consult_entry_click" | "phone_entry_click";
  source?: string;
  nickname?: string;
  phone?: string;
  propertyName?: string;
  createdAt?: string;
}
export const recordContactEvent = (item: ContactEvent) =>
  request<{ recorded: boolean }>("/v1/contact-events", { method: "POST", data: item });
export const listContactEvents = () => request<ContactEvent[]>("/v1/admin/contact-events");
export const listFavorites = () => request<string[]>("/v1/favorites");
export const setFavorite = (propertyId: string, favorite: boolean) =>
  request<{ propertyId: string; favorite: boolean }>(
    `/v1/favorites/${encodeURIComponent(propertyId)}`,
    { method: favorite ? "PUT" : "DELETE" },
  );
export interface Footprint { propertyId: string; viewedAt: string; viewCount: number }
export const recordFootprint = (propertyId: string) => request(`/v1/footprints/${encodeURIComponent(propertyId)}`, { method: "POST" });
export const listFootprints = () => request<Footprint[]>("/v1/footprints");
export const clearFootprints = () => request("/v1/footprints", { method: "DELETE" });
export interface SubscriptionTemplates { areaListing: string; appointment: string }
export const getSubscriptionTemplates = () => request<SubscriptionTemplates>("/v1/subscription-templates");
export const registerMessageSubscription = (templateId: string, area = "") => request("/v1/message-subscriptions", { method: "POST", data: { templateId, area } });

export interface WechatLoginResult {
  accessToken: string;
  role?: "user" | "admin";
  isDevMock?: boolean;
  testOnly?: boolean;
  phone?: string;
}

export interface PublicConfig {
  heroTitle: string;
  contactPhone: string;
  relatedPropertyRadiusKm?: number;
  locations: { id?: string; label: string; keyword: string; latitude?: number; longitude?: number; radiusKm?: number }[];
  featuredChannels: FeaturedChannel[];
  statusLabels?: { draft?: string; pendingReview?: string; published?: string; available?: string; rented?: string; offline?: string };
  pagePresentations?: Record<string, {
    copy?: Record<string, string>; searchPlaceholder?: string; operatorTitle?: string; operatorSubtitle?: string; eyebrow?: string;
    title?: string;
    subtitle?: string;
    backgroundImage?: string;
    channelTitle?: string;
    allChannelsText?: string;
    recommendationTitle?: string;
    viewAllText?: string;
    nearbyLabel?: string;
  }>;
  shareCard?: {
    titlePrefix?: string;
    titleFields?: string[];
    posterFields?: string[];
  };
  listingTitle?: {
    marketingPhrases?: string[];
    customerAllowedFields?: string[];
    enabledFields?: string[];
    customerDetailFields?: string[];
    customerShareFields?: string[];
    customerPosterFields?: string[];
    adminEnabledFields?: string[];
    listTagLayout?: "grouped" | "wrap";
    detailTagLayout?: "grouped" | "wrap";
    listTagOrder?: string[];
    listTagTopFields?: string[];
    detailTagOrder?: string[];
    detailTagTopFields?: string[];
  };
  propertyDetail?: {
    modules?: { key: string; title: string; enabled: boolean }[];
    nearby?: { radiusKm?: number; relation?: "distance" | "channel"; sort?: "distance" | "newest"; limit?: number };
    facilities?: { key: string; label: string; group: string; enabled: boolean }[];
  };
}

export const wechatLogin = (code: string) =>
  request<WechatLoginResult>("/v1/auth/wechat/login", {
    method: "POST",
    data: { code },
  });

export const bindWechatPhone = (code: string) =>
  request<{ phone: string }>("/v1/auth/wechat/phone", {
    method: "POST",
    data: { code },
  });

export const temporaryLogin = (phone: string, accessCode: string) =>
  request<WechatLoginResult & { phone: string; temporary: boolean }>(
    "/v1/auth/temporary/login",
    {
      method: "POST",
      data: { phone, accessCode },
    },
  );

export const testAccessLogin = (phone: string, accessCode: string) =>
  request<WechatLoginResult & { phone: string; testOnly: boolean }>(
    "/v1/auth/test-access/login",
    { method: "POST", data: { phone, accessCode } },
  );

export const refreshAuth = () =>
  request<{ accessToken: string; role: "user" | "admin" }>("/v1/auth/refresh", {
    method: "POST",
  });

export interface WechatUserProfile {
  id: string;
  nickname: string;
  avatarUrl: string;
  phone: string;
  status: string;
}

export const getWechatUserProfile = () =>
  request<WechatUserProfile>("/v1/auth/profile");

export const updateWechatUserProfile = (nickname: string, avatarUrl: string) =>
  request<WechatUserProfile>("/v1/auth/profile", {
    method: "PATCH",
    data: { nickname, avatarUrl },
  });

export const getPublicConfig = () => request<PublicConfig>("/v1/config/public");

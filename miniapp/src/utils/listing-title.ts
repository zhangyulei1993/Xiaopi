import type { Property } from "../api/properties";

export type ListingTitleField = "community" | "unit" | "floor" | "layout" | "rentType" | "area";

export const defaultListingTitleFields: ListingTitleField[] = [
  "community", "floor", "layout", "rentType", "area",
];

export const listingDoorplate = (item: Property) => {
  if (item.doorplateLabel) return String(item.doorplateLabel).trim();
  return [
    item.buildingNo,
    item.unitNo && (String(item.unitNo).includes("单元") ? item.unitNo : `${item.unitNo}单元`),
    item.roomNo && (String(item.roomNo).includes("室") ? item.roomNo : `${item.roomNo}室`),
  ].filter(Boolean).join("");
};

// The doorplate is deliberately absent here. It is rendered as a separate
// tag so configuration changes apply to old listings immediately and the
// core title remains readable on narrow cards.
export const publicListingTitle = (item: Property, fields?: string[]) => {
  // Public cards use the community as a stable title. Everything else belongs
  // to configurable tags, never to an auto-composed title.
  void fields;
  return String(item.community || item.title || "房源").trim();
};

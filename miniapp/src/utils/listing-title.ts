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

/**
 * Listing values are rendered as independent tags.  Legacy records used to
 * keep strings such as “三室一厅 · 整租” in layout, which made the layout tag
 * repeat the separately rendered rental-type tag.  Keep structural layout
 * information here and let rent type/source/fee each own their own tag.
 */
export const listingLayout = (item: Property) => String(item.layout || "")
  .split(/[·・|｜]/)
  .map((part) => part.trim())
  .filter((part) => part && !/^(整租|合租|直租|转租|有中介费|无中介费)$/.test(part))
  .join(" · ");

export const listingFee = (item: Property) => {
  const value = String(item.feeText || "");
  if (value.includes("无中介费")) return "无中介费";
  if (value.includes("有中介费")) return "有中介费";
  return "";
};

export const listingTagValue = (item: Property, field: string) => {
  if (field === "unit") return listingDoorplate(item);
  if (field === "floor") return String(item.floorInfo || "").trim();
  if (field === "layout") return listingLayout(item);
  if (field === "rentType") return item.rentType === "合租" ? "合租" : "整租";
  if (field === "listingSource") return item.listingSource === "转租" ? "转租" : "直租";
  if (field === "fee") return listingFee(item);
  if (field === "area") return item.areaSqm ? `${item.areaSqm}㎡` : "";
  return "";
};

export const uniqueListingTags = (values: string[]) => {
  const known = new Set<string>();
  return values.filter((value) => {
    const normalized = String(value || "").trim();
    if (!normalized || known.has(normalized)) return false;
    known.add(normalized);
    return true;
  });
};

// The doorplate is deliberately absent here. It is rendered as a separate
// tag so configuration changes apply to old listings immediately and the
// core title remains readable on narrow cards.
export const publicListingTitle = (item: Property, fields?: string[]) => {
  // Public cards use the community as a stable title. Everything else belongs
  // to configurable tags, never to an auto-composed title.
  void fields;
  const base = String(item.community || item.title || "房源").trim();
  const phrase = String(item.marketingPhrase || "").trim();
  return phrase ? `${base} · ${phrase}` : base;
};

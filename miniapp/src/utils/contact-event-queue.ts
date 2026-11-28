import { ApiError } from "../api/client";
import { recordContactEvent, type ContactEvent } from "../api/properties";
import { getSession } from "../auth/session";

type QueuedContact = Pick<ContactEvent, "propertyId" | "type">;
const legacyKey = "xiaopi_pending_contact_events";
const activeFlushes = new Set<string>();

function currentOwner(): string {
  const session = getSession();
  if (session?.role !== "user") return "";
  try {
    const payload = session.accessToken.split(".")[0].replace(/-/g, "+").replace(/_/g, "/");
    const padded = payload.padEnd(Math.ceil(payload.length / 4) * 4, "=");
    const bytes = new Uint8Array(uni.base64ToArrayBuffer(padded));
    const json = decodeURIComponent(Array.from(bytes, byte => "%" + byte.toString(16).padStart(2, "0")).join(""));
    const { sub } = JSON.parse(json);
    return typeof sub === "string" ? sub : "";
  } catch {
    return "";
  }
}

const queueKey = (owner: string) => legacyKey + ":" + encodeURIComponent(owner);
const sameEvent = (left: QueuedContact, right: QueuedContact) =>
  left.propertyId === right.propertyId && left.type === right.type;

function readQueue(owner: string): QueuedContact[] {
  const items = uni.getStorageSync(queueKey(owner));
  return Array.isArray(items) ? items : [];
}

function writeQueue(owner: string, items: QueuedContact[]) {
  if (items.length) uni.setStorageSync(queueKey(owner), items);
  else uni.removeStorageSync(queueKey(owner));
}

function canRetry(error: unknown) {
  return !(error instanceof ApiError) || [0, 401, 408, 429].includes(error.statusCode) || error.statusCode >= 500;
}

export function recordOrQueueContact(event: QueuedContact) {
  if (getSession()?.role !== "user") return;
  const owner = currentOwner();
  void recordContactEvent({ ...event, source: "detail" }).catch(error => {
    if (!owner || !canRetry(error)) return;
    const queued = readQueue(owner).filter(saved => !sameEvent(saved, event));
    writeQueue(owner, [...queued, event].slice(-10));
  });
}

export async function flushPendingContactEvents() {
  // Older records carry no account identity and cannot be attributed safely.
  if (uni.getStorageSync(legacyKey)) uni.removeStorageSync(legacyKey);
  const owner = currentOwner();
  if (!owner || activeFlushes.has(owner)) return;
  activeFlushes.add(owner);
  try {
    for (const event of readQueue(owner)) {
      if (currentOwner() !== owner) break;
      try {
        await recordContactEvent({ ...event, source: "detail" });
      } catch (error) {
        if (canRetry(error)) break;
      }
      // Re-read before removing the sent event so queued clicks from this
      // account that arrived during the request remain available.
      writeQueue(owner, readQueue(owner).filter(saved => !sameEvent(saved, event)));
    }
  } finally {
    activeFlushes.delete(owner);
  }
}

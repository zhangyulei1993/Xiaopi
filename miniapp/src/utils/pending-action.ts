export type PendingAction = {
  type: "appointment" | "favorite" | "area_subscription";
  propertyId?: string;
  area?: string;
};

const KEY = "xiaopi_pending_action";

export const savePendingAction = (action: PendingAction) =>
  uni.setStorageSync(KEY, action);

export const peekPendingAction = (): PendingAction | null => {
  const action = uni.getStorageSync(KEY) as PendingAction | undefined;
  return action?.type ? action : null;
};

export const clearPendingAction = () => uni.removeStorageSync(KEY);

export const takePendingAction = (): PendingAction | null => {
  const action = peekPendingAction();
  if (!action) return null;
  clearPendingAction();
  return action;
};

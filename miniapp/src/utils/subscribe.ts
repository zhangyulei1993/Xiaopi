export const requestMessageSubscription = (templateId: string) => new Promise<boolean>((resolve) => {
  if (!templateId) { resolve(false); return; }
  uni.requestSubscribeMessage({
    tmplIds: [templateId],
    success: (result) => resolve(String((result as unknown as Record<string, unknown>)[templateId]) === "accept"),
    fail: () => resolve(false),
  });
});

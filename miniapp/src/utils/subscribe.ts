export const requestMessageSubscription = (templateId: string) => new Promise<boolean>((resolve) => {
  if (!templateId) {
    uni.showToast({ title: "订阅模板未配置，请联系管理员", icon: "none" });
    resolve(false);
    return;
  }
  if (typeof uni.requestSubscribeMessage !== "function") {
    uni.showToast({ title: "当前微信版本不支持消息订阅", icon: "none" });
    resolve(false);
    return;
  }
  uni.requestSubscribeMessage({
    tmplIds: [templateId],
    success: (result) => {
      const status = String((result as unknown as Record<string, unknown>)[templateId] || "");
      const accepted = status === "accept";
      if (!accepted) uni.showToast({ title: status === "ban" ? "请在微信设置中开启订阅权限" : "未开启消息订阅", icon: "none" });
      resolve(accepted);
    },
    fail: (error) => {
      const reason = String(error?.errMsg || "");
      uni.showToast({ title: reason.includes("cancel") ? "已取消消息订阅" : "微信订阅授权失败，请稍后重试", icon: "none" });
      resolve(false);
    },
  });
});

export const callPhone = (phoneNumber: string, missingMessage = "联系电话尚未配置") => {
  if (!phoneNumber) {
    uni.showToast({ title: missingMessage, icon: "none" });
    return;
  }
  uni.makePhoneCall({
    phoneNumber,
    fail: (error) => {
      const message = String(error?.errMsg || "").toLowerCase();
      if (message.includes("cancel")) return;
      uni.showToast({ title: "暂时无法拨号，请稍后重试", icon: "none" });
    },
  });
};

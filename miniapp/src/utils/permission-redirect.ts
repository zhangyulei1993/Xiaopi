// Lifecycle guards can run before the navigation that opened the page has
// completed. Defer the tab switch and handle a rejected transition explicitly.
export function returnToMyPage() {
  setTimeout(() => {
    uni.switchTab({
      url: '/pages/me/index',
      fail: () => { uni.reLaunch({ url: '/pages/me/index' }); },
    });
  }, 300);
}

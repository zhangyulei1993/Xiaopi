import { onUnload } from "@dcloudio/uni-app";

let foreground = true;
const foregroundListeners = new Set<() => void>();
const backgroundListeners = new Set<() => void>();

export function setUploadForeground(active: boolean) {
  foreground = active;
  for (const listener of [...(active ? foregroundListeners : backgroundListeners)]) listener();
}

export function useForegroundUpload(onPaused: () => void, onResumed: () => void) {
  let disposed = false;
  const cancelListeners = new Set<() => void>();
  onUnload(() => {
    disposed = true;
    for (const cancel of [...cancelListeners]) cancel();
  });

  const waitForForeground = () => new Promise<void>((resolve, reject) => {
    if (disposed) { reject(new Error("已退出录入")); return; }
    if (foreground) { resolve(); return; }
    onPaused();
    const cleanup = () => {
      foregroundListeners.delete(resume);
      cancelListeners.delete(cancel);
    };
    const resume = () => { cleanup(); onResumed(); resolve(); };
    const cancel = () => { cleanup(); reject(new Error("已退出录入")); };
    foregroundListeners.add(resume);
    cancelListeners.add(cancel);
  });

  return async function upload<T>(start: (resolve: (value: T) => void, reject: (error: Error) => void) => UniApp.UploadTask): Promise<T> {
    for (;;) {
      await waitForForeground();
      if (disposed) throw new Error("已退出录入");
      if (!foreground) continue;
      let paused = false;
      let result: T;
      try {
        result = await new Promise<T>((resolve, reject) => {
          const cleanup = () => {
            backgroundListeners.delete(pause);
            cancelListeners.delete(cancel);
          };
          const pause = () => {
            paused = true;
            onPaused();
            cleanup();
            reject(new Error("上传已暂停"));
            task.abort();
          };
          const cancel = () => {
            cleanup();
            reject(new Error("已退出录入"));
            task.abort();
          };
          const task = start(value => { cleanup(); resolve(value); }, error => { cleanup(); reject(error); });
          backgroundListeners.add(pause);
          cancelListeners.add(cancel);
        });
      } catch (error) {
        if (paused && !disposed) {
          if (foreground) onResumed();
          continue;
        }
        throw error;
      }
      return result;
    }
  };
}

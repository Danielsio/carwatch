import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { pushApi } from "@/lib/api";

export interface PushState {
  supported: boolean;
  permission: NotificationPermission;
  subscribed: boolean;
}

const unsupportedState: PushState = {
  supported: false,
  permission: "denied",
  subscribed: false,
};

export function usePushSubscription(enabled: boolean) {
  const qc = useQueryClient();

  const { data: pushState } = useQuery({
    queryKey: ["push-state"],
    queryFn: async (): Promise<PushState> => {
      if (!("serviceWorker" in navigator) || !("PushManager" in window)) {
        return unsupportedState;
      }
      const permission = Notification.permission;
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.getSubscription();
      return { supported: true, permission, subscribed: !!sub };
    },
    enabled,
    staleTime: Infinity,
  });

  const subscribe = useMutation({
    mutationFn: async () => {
      const { public_key } = await pushApi.vapidKey();
      const permission = await Notification.requestPermission();
      if (permission !== "granted") throw new Error("Permission denied");

      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(public_key),
      });
      await pushApi.subscribe(sub.toJSON());
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["push-state"] }),
  });

  const unsubscribe = useMutation({
    mutationFn: async () => {
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.getSubscription();
      if (sub) {
        await pushApi.unsubscribe(sub.endpoint);
        await sub.unsubscribe();
      }
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["push-state"] }),
  });

  return {
    pushState: pushState ?? unsupportedState,
    subscribe,
    unsubscribe,
  };
}

// Returns a Uint8Array explicitly backed by an ArrayBuffer (not the wider
// ArrayBufferLike the bare `Uint8Array` type now defaults to in TS 5.9's
// lib.dom.d.ts). PushManager.subscribe's applicationServerKey wants a
// BufferSource, which a Uint8Array<ArrayBufferLike> — possibly SharedArrayBuffer
// backed — does not satisfy; pinning the buffer type makes the assignment valid.
function urlBase64ToUint8Array(base64String: string): Uint8Array<ArrayBuffer> {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding)
    .replace(/-/g, "+")
    .replace(/_/g, "/");
  const rawData = atob(base64);
  const bytes = new Uint8Array(new ArrayBuffer(rawData.length));
  for (let i = 0; i < rawData.length; i++) {
    bytes[i] = rawData.charCodeAt(i);
  }
  return bytes;
}

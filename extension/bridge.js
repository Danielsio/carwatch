// Bridge script running in ISOLATED world.
// Listens for auth token messages from the MAIN world content script
// and saves them to chrome.storage.sync.

window.addEventListener("message", (event) => {
  if (event.source !== window) return;
  if (event.data?.type === "CW_AUTH_TOKEN" && event.data.token) {
    chrome.storage.sync.set({ authToken: event.data.token });
  }
});

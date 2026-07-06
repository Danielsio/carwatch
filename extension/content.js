// Content script injected into CarWatch web app pages.
// Extracts the Firebase auth token from IndexedDB and sends it to the
// background service worker so the extension can authenticate API calls.

const POLL_INTERVAL_MS = 30000;

async function extractToken() {
  try {
    const dbs = await indexedDB.databases();
    const firebaseDB = dbs.find(
      (db) => db.name && db.name.startsWith("firebaseLocalStorage"),
    );
    if (!firebaseDB) return null;

    return new Promise((resolve) => {
      const req = indexedDB.open(firebaseDB.name);
      req.onsuccess = () => {
        const db = req.result;
        const storeNames = Array.from(db.objectStoreNames);
        const storeName = storeNames.find((s) =>
          s.includes("firebaseLocalStorage"),
        );
        if (!storeName) {
          db.close();
          resolve(null);
          return;
        }

        const tx = db.transaction(storeName, "readonly");
        const store = tx.objectStore(storeName);
        const getAll = store.getAll();
        getAll.onsuccess = () => {
          db.close();
          for (const entry of getAll.result) {
            const val = entry?.value;
            if (val?.stsTokenManager?.accessToken) {
              resolve(val.stsTokenManager.accessToken);
              return;
            }
          }
          resolve(null);
        };
        getAll.onerror = () => {
          db.close();
          resolve(null);
        };
      };
      req.onerror = () => resolve(null);
    });
  } catch {
    return null;
  }
}

async function syncToken() {
  const token = await extractToken();
  if (token) {
    chrome.storage.sync.set({ authToken: token });
  }
}

syncToken();
setInterval(syncToken, POLL_INTERVAL_MS);

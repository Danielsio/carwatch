type AuthTokenGetter = (forceRefresh?: boolean) => Promise<string | null>;

let getTokenImpl: AuthTokenGetter = async () => null;

export function setAuthTokenGetter(fn: AuthTokenGetter) {
  getTokenImpl = fn;
}

export async function getAuthToken(forceRefresh?: boolean): Promise<string | null> {
  return getTokenImpl(forceRefresh);
}

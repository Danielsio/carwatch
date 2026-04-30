type AuthTokenGetter = () => Promise<string | null>;

let getTokenImpl: AuthTokenGetter = async () => null;

export function setAuthTokenGetter(fn: AuthTokenGetter) {
  getTokenImpl = fn;
}

export async function getAuthToken(): Promise<string | null> {
  return getTokenImpl();
}

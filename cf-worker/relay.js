const ALLOWED_HOSTS = ["www.yad2.co.il", "gw.yad2.co.il", "yad2.co.il"];

const FORWARDED_HEADERS = [
  "User-Agent",
  "Accept",
  "Accept-Language",
  "Accept-Encoding",
  "DNT",
  "Cache-Control",
];

export default {
  async fetch(request, env) {
    const secret = request.headers.get("X-Relay-Secret");
    if (!secret || secret !== env.RELAY_SECRET) {
      return new Response("unauthorized", { status: 401 });
    }

    const url = new URL(request.url);
    const target = url.searchParams.get("target");
    if (!target) {
      return new Response("missing target parameter", { status: 400 });
    }

    let targetURL;
    try {
      targetURL = new URL(target);
    } catch {
      return new Response("invalid target URL", { status: 400 });
    }

    if (!ALLOWED_HOSTS.includes(targetURL.hostname)) {
      return new Response("target host not allowed", { status: 403 });
    }

    const headers = new Headers();
    for (const h of FORWARDED_HEADERS) {
      const val = request.headers.get(h);
      if (val) headers.set(h, val);
    }

    try {
      const response = await fetch(target, {
        method: "GET",
        headers: headers,
        redirect: "follow",
      });

      const responseHeaders = new Headers();
      responseHeaders.set(
        "Content-Type",
        response.headers.get("Content-Type") || "application/octet-stream",
      );

      return new Response(response.body, {
        status: response.status,
        headers: responseHeaders,
      });
    } catch (err) {
      return new Response("relay fetch failed: " + err.message, {
        status: 502,
      });
    }
  },
};

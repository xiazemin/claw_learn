const BASE_URL = "./api/wps-openapi";

export interface WpsApiRequestOptions extends Omit<RequestInit, "method" | "body" | "headers" | "credentials"> {
  params?: Record<string, string | string[] | undefined>;
  headers?: Record<string, string>;
}

function buildUrl(path: string, params?: WpsApiRequestOptions["params"]): string {
  const url = `${BASE_URL}${path}`;
  if (!params) return url;

  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined) continue;
    if (Array.isArray(value)) {
      searchParams.set(key, value.join(","));
    } else {
      searchParams.set(key, value);
    }
  }
  const qs = searchParams.toString();
  return qs ? `${url}?${qs}` : url;
}

async function request<T = unknown>(
  method: string,
  path: string,
  options: WpsApiRequestOptions & { body?: unknown } = {},
): Promise<T> {
  const { params, headers: customHeaders, body, ...fetchOptions } = options;

  const url = buildUrl(path, params);
  const headers: Record<string, string> = {
    ...customHeaders,
  };

  if (body !== undefined && body !== null) {
    headers["Content-Type"] = "application/json";
  }

  const res = await fetch(url, {
    method,
    headers,
    credentials: "include",
    body: body !== undefined && body !== null ? JSON.stringify(body) : undefined,
    ...fetchOptions,
  });

  if (!res.ok) {
    const err = new Error(`WPS API error: ${res.status}`) as Error & { status: number };
    err.status = res.status;
    throw err;
  }

  return res.json() as Promise<T>;
}

export const wpsApi = {
  get<T = unknown>(path: string, options?: WpsApiRequestOptions) {
    return request<T>("GET", path, options);
  },
  post<T = unknown>(path: string, body?: unknown, options?: WpsApiRequestOptions) {
    return request<T>("POST", path, { ...options, body });
  },
  put<T = unknown>(path: string, body?: unknown, options?: WpsApiRequestOptions) {
    return request<T>("PUT", path, { ...options, body });
  },
  delete<T = unknown>(path: string, options?: WpsApiRequestOptions) {
    return request<T>("DELETE", path, options);
  },
};

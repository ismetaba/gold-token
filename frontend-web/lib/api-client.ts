/**
 * GOLD API Client
 *
 * All calls go through {NEXT_PUBLIC_API_URL}/{arena}/v1/{resource}.
 * In POC mode, calls fall back to mock data when the backend is unreachable.
 */

import type {
  AdminKycReviewPayload,
  AdminStats,
  AdminTokenOpPayload,
  AdminUserRow,
  ApiResponse,
  AuthTokens,
  ComplianceFlag,
  ComplianceFlagStatus,
  FeeConfigUpdatePayload,
  FeeRule,
  GoldPrice,
  KycDocument,
  KycSession,
  Order,
  PaginatedResponse,
  ProofOfReserve,
  SanctionsScreeningResult,
  SystemHealthSummary,
  TokenOperation,
  User,
  Wallet,
} from "./types";
import { ADMIN_MOCK, MOCK } from "./mock-data";

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

const POC_MOCK = process.env.NEXT_PUBLIC_USE_MOCK === "true";

/**
 * Approximate TRY → USD conversion rate used for display-only estimates.
 * POC placeholder; in production this would come from an FX feed.
 */
export const TRY_PER_USD = 33;

/** Convert a TRY amount to an approximate USD amount (display only). */
export function tryToUsd(amountTRY: number): number {
  return amountTRY / TRY_PER_USD;
}

const TOKEN_KEY = "gold_access_token";
const REFRESH_KEY = "gold_refresh_token";
const EXPIRES_KEY = "gold_token_expires";

// ── Helpers ──────────────────────────────────────────────────────────────────

const KNOWN_ARENAS = new Set(["tr", "ch", "ae", "eu"]);
const DEFAULT_ARENA = "tr";

// getArena reads the selected jurisdiction from localStorage but allowlists it
// against the known set before it is interpolated into the request URL/header,
// so a tampered value cannot inject path segments or arbitrary header content.
function getArena(): string {
  if (typeof window === "undefined") return DEFAULT_ARENA;
  const a = (localStorage.getItem("gold_arena") ?? DEFAULT_ARENA).toLowerCase();
  return KNOWN_ARENAS.has(a) ? a : DEFAULT_ARENA;
}

function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEY);
}

function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(REFRESH_KEY);
}

/**
 * Optional callback invoked when a token refresh ultimately fails, so the
 * auth layer can clear its in-memory state (the tokens are already cleared
 * from localStorage here). Registered by AuthContext via setOnAuthFailure.
 */
let onAuthFailure: (() => void) | null = null;

export function setOnAuthFailure(cb: (() => void) | null): void {
  onAuthFailure = cb;
}

/**
 * Single-flight token refresh. Concurrent 401s share the same in-flight
 * refresh promise instead of stampeding the refresh endpoint. Resolves to
 * the new access token, or null if refresh failed.
 */
let refreshInFlight: Promise<string | null> | null = null;

function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight;

  refreshInFlight = (async () => {
    const refreshToken = getRefreshToken();
    if (!refreshToken) return null;
    try {
      const res = await authApi.refresh(refreshToken);
      const tokens = res.data;
      if (typeof window !== "undefined") {
        localStorage.setItem(TOKEN_KEY, tokens.accessToken);
        localStorage.setItem(REFRESH_KEY, tokens.refreshToken);
        localStorage.setItem(EXPIRES_KEY, String(tokens.expiresAt));
      }
      return tokens.accessToken;
    } catch {
      // Refresh failed → clear stored tokens (logout).
      if (typeof window !== "undefined") {
        localStorage.removeItem(TOKEN_KEY);
        localStorage.removeItem(REFRESH_KEY);
        localStorage.removeItem(EXPIRES_KEY);
      }
      onAuthFailure?.();
      return null;
    } finally {
      refreshInFlight = null;
    }
  })();

  return refreshInFlight;
}

function apiUrl(path: string): string {
  const arena = getArena();
  return `${API_BASE}/${arena}/v1${path}`;
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  extraHeaders?: Record<string, string>,
  isRetry = false
): Promise<T> {
  const token = getAccessToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-GOLD-Arena": getArena().toUpperCase(),
    ...extraHeaders,
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(apiUrl(path), {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });

  if (!res.ok) {
    // Access token expired: attempt a single refresh-and-retry. Concurrent
    // 401s share one refresh (single-flight) via refreshAccessToken().
    // Skip for the refresh endpoint itself to avoid recursion.
    if (
      res.status === 401 &&
      !isRetry &&
      getRefreshToken() &&
      path !== "/auth/refresh"
    ) {
      const newToken = await refreshAccessToken();
      if (newToken) {
        return request<T>(method, path, body, extraHeaders, true);
      }
    }

    const err = await res.json().catch(() => ({}));
    throw new ApiClientError(
      err?.error?.code ?? "GOLD.UNKNOWN",
      err?.error?.message ?? `HTTP ${res.status}`,
      res.status
    );
  }

  return res.json() as Promise<T>;
}

export class ApiClientError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number
  ) {
    super(message);
    this.name = "ApiClientError";
  }
}

// ── Auth ─────────────────────────────────────────────────────────────────────

export const authApi = {
  async register(params: {
    email: string;
    password: string;
    fullName: string;
    arena: string;
  }): Promise<ApiResponse<{ user: User; tokens: AuthTokens }>> {
    if (POC_MOCK) {
      await delay(600);
      const user: User = {
        id: uuid(),
        email: params.email,
        fullName: params.fullName,
        arena: params.arena as User["arena"],
        kycStatus: "not_started",
        createdAt: new Date().toISOString(),
      };
      const tokens = mockTokens();
      return { data: { user, tokens }, meta: { request_id: uuid() } };
    }
    return request("POST", "/auth/register", params);
  },

  async login(params: {
    email: string;
    password: string;
  }): Promise<ApiResponse<{ user: User; tokens: AuthTokens }>> {
    if (POC_MOCK) {
      await delay(600);
      if (params.password.length < 4) throw new ApiClientError("GOLD.AUTH.001", "Invalid credentials", 401);
      return {
        data: { user: MOCK.user, tokens: mockTokens() },
        meta: { request_id: uuid() },
      };
    }
    return request("POST", "/auth/login", params);
  },

  async refresh(refreshToken: string): Promise<ApiResponse<AuthTokens>> {
    if (POC_MOCK) {
      await delay(200);
      return { data: mockTokens(), meta: { request_id: uuid() } };
    }
    return request("POST", "/auth/refresh", { refreshToken });
  },

  async me(): Promise<ApiResponse<User>> {
    if (POC_MOCK) {
      await delay(200);
      return { data: MOCK.user, meta: { request_id: uuid() } };
    }
    return request("GET", "/auth/me");
  },
};

// ── KYC ──────────────────────────────────────────────────────────────────────

export const kycApi = {
  async getSession(): Promise<ApiResponse<KycSession | null>> {
    if (POC_MOCK) {
      await delay(400);
      return { data: MOCK.kycSession, meta: { request_id: uuid() } };
    }
    return request("GET", "/kyc/session");
  },

  async startSession(): Promise<ApiResponse<KycSession>> {
    if (POC_MOCK) {
      await delay(600);
      const session: KycSession = {
        id: uuid(),
        userId: MOCK.user.id,
        status: "pending",
        provider: "sumsub",
        documents: [],
      };
      return { data: session, meta: { request_id: uuid() } };
    }
    return request("POST", "/kyc/sessions");
  },

  async uploadDocument(
    sessionId: string,
    docType: KycDocument["type"],
    _file: File
  ): Promise<ApiResponse<KycDocument>> {
    if (POC_MOCK) {
      await delay(1200);
      const doc: KycDocument = {
        id: uuid(),
        type: docType,
        status: "uploaded",
        uploadedAt: new Date().toISOString(),
      };
      return { data: doc, meta: { request_id: uuid() } };
    }
    // Real: multipart form upload
    const fd = new FormData();
    fd.append("file", _file);
    fd.append("type", docType);
    const token = getAccessToken();
    const res = await fetch(apiUrl(`/kyc/sessions/${sessionId}/documents`), {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        "X-GOLD-Arena": getArena().toUpperCase(),
      },
      body: fd,
    });
    if (!res.ok) throw new ApiClientError("GOLD.KYC.001", "Upload failed", res.status);
    return res.json();
  },

  async submitSession(sessionId: string): Promise<ApiResponse<KycSession>> {
    if (POC_MOCK) {
      await delay(700);
      return {
        data: { ...MOCK.kycSession, id: sessionId, status: "in_review" },
        meta: { request_id: uuid() },
      };
    }
    return request("POST", `/kyc/sessions/${sessionId}/submit`);
  },
};

// ── Wallet ────────────────────────────────────────────────────────────────────

export const walletApi = {
  async getWallet(): Promise<ApiResponse<Wallet>> {
    if (POC_MOCK) {
      await delay(300);
      return { data: MOCK.wallet, meta: { request_id: uuid() } };
    }
    return request("GET", "/wallet");
  },
};

// ── Price ─────────────────────────────────────────────────────────────────────

export const priceApi = {
  async getCurrentPrice(): Promise<ApiResponse<{ price: GoldPrice }>> {
    if (POC_MOCK) {
      await delay(200);
      // Simulate slight price movement
      const base = 3150 + Math.random() * 50;
      return {
        data: {
          price: {
            pricePerGramTRY: (base).toFixed(2),
            pricePerGramUSD: tryToUsd(base).toFixed(4),
            pricePerOzUSD: (tryToUsd(base) * 31.1035).toFixed(2),
            source: "LBMA + Chainlink",
            updatedAt: new Date().toISOString(),
          },
        },
        meta: { request_id: uuid() },
      };
    }
    return request("GET", "/price/current");
  },
};

// ── Orders ───────────────────────────────────────────────────────────────────

export const ordersApi = {
  async createBuyOrder(params: {
    amountGrams: string;
    idempotencyKey: string;
  }): Promise<ApiResponse<Order>> {
    if (POC_MOCK) {
      await delay(800);
      const price = MOCK.price;
      const grams = parseFloat(params.amountGrams);
      const order: Order = {
        id: uuid(),
        userId: MOCK.user.id,
        type: "buy",
        status: "PAYMENT_PENDING",
        amountGrams: params.amountGrams,
        amountWei: (grams * 1e18).toFixed(0),
        pricePerGramTRY: price.pricePerGramTRY,
        totalTRY: (grams * parseFloat(price.pricePerGramTRY)).toFixed(2),
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      return { data: order, meta: { request_id: uuid() } };
    }
    return request("POST", "/orders/buy", params, {
      "Idempotency-Key": params.idempotencyKey,
    });
  },

  async simulatePayment(orderId: string): Promise<ApiResponse<Order>> {
    if (POC_MOCK) {
      await delay(1500);
      const order: Order = {
        ...MOCK.orders[0],
        id: orderId,
        status: "MINT_EXECUTED",
        completedAt: new Date().toISOString(),
      };
      return { data: order, meta: { request_id: uuid() } };
    }
    return request("POST", `/orders/${orderId}/payment-callback`);
  },

  async createSellOrder(params: {
    amountGrams: string;
    idempotencyKey: string;
    ibanOrAddress: string;
  }): Promise<ApiResponse<Order>> {
    if (POC_MOCK) {
      await delay(800);
      const price = MOCK.price;
      const grams = parseFloat(params.amountGrams);
      const order: Order = {
        id: uuid(),
        userId: MOCK.user.id,
        type: "redeem_cash",
        status: "CREATED",
        amountGrams: params.amountGrams,
        amountWei: (grams * 1e18).toFixed(0),
        pricePerGramTRY: price.pricePerGramTRY,
        totalTRY: (grams * parseFloat(price.pricePerGramTRY)).toFixed(2),
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      return { data: order, meta: { request_id: uuid() } };
    }
    return request("POST", "/orders/redeem", params, {
      "Idempotency-Key": params.idempotencyKey,
    });
  },

  async getOrder(orderId: string): Promise<ApiResponse<Order>> {
    if (POC_MOCK) {
      await delay(300);
      return {
        data: { ...MOCK.orders[0], id: orderId },
        meta: { request_id: uuid() },
      };
    }
    return request("GET", `/orders/${orderId}`);
  },

  async listOrders(cursor?: string): Promise<PaginatedResponse<Order>> {
    if (POC_MOCK) {
      await delay(400);
      return {
        data: MOCK.orders,
        meta: { request_id: uuid(), hasMore: false },
      };
    }
    const qs = cursor ? `?cursor=${cursor}` : "";
    return request("GET", `/orders${qs}`);
  },
};

// ── PoR ──────────────────────────────────────────────────────────────────────

export const porApi = {
  async getLatest(): Promise<ApiResponse<ProofOfReserve>> {
    if (POC_MOCK) {
      await delay(300);
      return { data: MOCK.por, meta: { request_id: uuid() } };
    }
    return request("GET", "/por/latest");
  },

  async list(): Promise<PaginatedResponse<ProofOfReserve>> {
    if (POC_MOCK) {
      await delay(400);
      return {
        data: [MOCK.por],
        meta: { request_id: uuid(), hasMore: false },
      };
    }
    return request("GET", "/por");
  },
};

// ── Admin ─────────────────────────────────────────────────────────────────────

/**
 * Returns headers required by backend admin endpoints.
 * KYC admin routes expect X-Admin-Secret; PoR routes expect X-Admin-Token.
 *
 * WARNING: NEXT_PUBLIC_* environment variables are inlined into the
 * client-side JavaScript bundle at build time and are fully readable by
 * anyone who loads the app. They are NOT secret. Real admin credentials
 * MUST NOT be shipped this way — the values here are POC placeholders only.
 * A production design would proxy these calls through a server-side route
 * handler that holds the real secret. (Out of scope for this change.)
 */
function adminHeaders(variant: "kyc" | "por" | "general" = "general"): Record<string, string> {
  const secret = process.env.NEXT_PUBLIC_ADMIN_SECRET ?? "";
  const token  = process.env.NEXT_PUBLIC_ADMIN_TOKEN  ?? "";
  if (variant === "kyc")     return { "X-Admin-Secret": secret };
  if (variant === "por")     return { "X-Admin-Token": token };
  // general: send both so the backend can accept whichever it needs
  return { "X-Admin-Secret": secret, "X-Admin-Token": token };
}

export const adminApi = {
  async getStats(): Promise<ApiResponse<AdminStats>> {
    if (POC_MOCK) {
      await delay(400);
      return { data: ADMIN_MOCK.stats, meta: { request_id: uuid() } };
    }
    return request("GET", "/admin/stats", undefined, adminHeaders());
  },

  async listUsers(params?: { kycStatus?: string; cursor?: string }): Promise<PaginatedResponse<AdminUserRow>> {
    if (POC_MOCK) {
      await delay(500);
      let users = ADMIN_MOCK.users;
      if (params?.kycStatus) {
        users = users.filter((u) => u.kycStatus === params.kycStatus);
      }
      return { data: users, meta: { request_id: uuid(), hasMore: false, total: users.length } };
    }
    const qs = new URLSearchParams(params as Record<string, string> ?? {}).toString();
    return request("GET", `/admin/users${qs ? `?${qs}` : ""}`, undefined, adminHeaders());
  },

  async reviewKyc(kycSessionId: string, payload: AdminKycReviewPayload): Promise<ApiResponse<{ success: boolean }>> {
    if (POC_MOCK) {
      await delay(700);
      return { data: { success: true }, meta: { request_id: uuid() } };
    }
    return request("PATCH", `/admin/kyc/${kycSessionId}/review`, payload, adminHeaders("kyc"));
  },

  async listTokenOps(cursor?: string): Promise<PaginatedResponse<TokenOperation>> {
    if (POC_MOCK) {
      await delay(400);
      return { data: ADMIN_MOCK.tokenOps, meta: { request_id: uuid(), hasMore: false } };
    }
    const qs = cursor ? `?cursor=${cursor}` : "";
    return request("GET", `/admin/token-ops${qs}`, undefined, adminHeaders());
  },

  async submitTokenOp(payload: AdminTokenOpPayload): Promise<ApiResponse<TokenOperation>> {
    if (POC_MOCK) {
      await delay(1000);
      const grams = parseFloat(payload.amountGrams);
      const op: TokenOperation = {
        id: uuid(),
        type: payload.type,
        initiatedBy: MOCK.user.id,
        targetUserId: payload.targetUserId,
        toAddress: payload.toAddress,
        amountGrams: payload.amountGrams,
        amountWei: (grams * 1e18).toFixed(0),
        status: "pending",
        createdAt: new Date().toISOString(),
      };
      return { data: op, meta: { request_id: uuid() } };
    }
    return request("POST", "/admin/token-ops", payload, adminHeaders());
  },

  async triggerPorAttestation(): Promise<ApiResponse<ProofOfReserve>> {
    if (POC_MOCK) {
      await delay(1500);
      const newPor: ProofOfReserve = {
        ...ADMIN_MOCK.porHistory[0],
        id: `por-${Date.now()}`,
        cycleId: `cycle-${Date.now()}`,
        attestedAt: new Date().toISOString(),
      };
      return { data: newPor, meta: { request_id: uuid() } };
    }
    return request("POST", "/admin/por/attest", undefined, adminHeaders("por"));
  },

  async listPorHistory(): Promise<PaginatedResponse<ProofOfReserve>> {
    if (POC_MOCK) {
      await delay(400);
      return { data: ADMIN_MOCK.porHistory, meta: { request_id: uuid(), hasMore: false } };
    }
    return request("GET", "/admin/por", undefined, adminHeaders("por"));
  },

  // ── Compliance ──────────────────────────────────────────────────────────────

  async listSanctionsScreenings(): Promise<PaginatedResponse<SanctionsScreeningResult>> {
    if (POC_MOCK) {
      await delay(400);
      return { data: ADMIN_MOCK.sanctionsScreenings, meta: { request_id: uuid(), hasMore: false } };
    }
    return request("GET", "/admin/compliance/screenings", undefined, adminHeaders());
  },

  async listComplianceFlags(status?: ComplianceFlagStatus): Promise<PaginatedResponse<ComplianceFlag>> {
    if (POC_MOCK) {
      await delay(400);
      let flags = ADMIN_MOCK.complianceFlags;
      if (status) flags = flags.filter((f) => f.status === status);
      return { data: flags, meta: { request_id: uuid(), hasMore: false } };
    }
    const qs = status ? `?status=${status}` : "";
    return request("GET", `/admin/compliance/flags${qs}`, undefined, adminHeaders());
  },

  async updateComplianceFlagStatus(
    flagId: string,
    status: ComplianceFlagStatus,
    note?: string
  ): Promise<ApiResponse<ComplianceFlag>> {
    if (POC_MOCK) {
      await delay(600);
      const flag = ADMIN_MOCK.complianceFlags.find((f) => f.id === flagId);
      if (!flag) throw new ApiClientError("GOLD.COMPLIANCE.001", "Flag not found", 404);
      const updated = { ...flag, status, updatedAt: new Date().toISOString() };
      return { data: updated, meta: { request_id: uuid() } };
    }
    return request("PATCH", `/admin/compliance/flags/${flagId}`, { status, note }, adminHeaders());
  },

  // ── System Health ───────────────────────────────────────────────────────────

  async getSystemHealth(): Promise<ApiResponse<SystemHealthSummary>> {
    if (POC_MOCK) {
      await delay(300);
      return { data: ADMIN_MOCK.systemHealth, meta: { request_id: uuid() } };
    }
    return request("GET", "/admin/system/health", undefined, adminHeaders());
  },

  // ── Fee Configuration ───────────────────────────────────────────────────────

  async listFeeRules(): Promise<PaginatedResponse<FeeRule>> {
    if (POC_MOCK) {
      await delay(300);
      return { data: ADMIN_MOCK.feeRules, meta: { request_id: uuid(), hasMore: false } };
    }
    return request("GET", "/admin/fees", undefined, adminHeaders());
  },

  async updateFeeRule(ruleId: string, payload: FeeConfigUpdatePayload): Promise<ApiResponse<FeeRule>> {
    if (POC_MOCK) {
      await delay(700);
      const rule = ADMIN_MOCK.feeRules.find((r) => r.id === ruleId);
      if (!rule) throw new ApiClientError("GOLD.FEE.001", "Fee rule not found", 404);
      const updated: FeeRule = {
        ...rule,
        feeBps: payload.feeBps,
        minFeeTRY: payload.minFeeTRY,
        maxFeeTRY: payload.maxFeeTRY,
        updatedAt: new Date().toISOString(),
        updatedBy: "usr-demo-001",
      };
      return { data: updated, meta: { request_id: uuid() } };
    }
    return request("PATCH", `/admin/fees/${ruleId}`, payload, adminHeaders());
  },
};

// ── Utilities ─────────────────────────────────────────────────────────────────

function delay(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}

function uuid(): string {
  return crypto.randomUUID ? crypto.randomUUID() : Math.random().toString(36).slice(2);
}

function mockTokens(): AuthTokens {
  return {
    accessToken: "mock.jwt.token",
    refreshToken: "mock.refresh.token",
    expiresAt: Date.now() + 15 * 60 * 1000,
  };
}

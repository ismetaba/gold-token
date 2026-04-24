// ─── Core types aligned with GOLD backend spec ───────────────────────────────

export type Arena = "tr" | "ch" | "ae" | "eu";

export type UserRole = "user" | "admin" | "superadmin";

// ── Auth ─────────────────────────────────────────────────────────────────────
export interface User {
  id: string;
  email: string;
  fullName: string;
  arena: Arena;
  kycStatus: KycStatus;
  role?: UserRole;
  createdAt: string;
}

export interface AuthTokens {
  accessToken: string;  // JWT RS256, 15m TTL
  refreshToken: string; // 7 days, rotating
  expiresAt: number;    // unix ms
}

// ── KYC ──────────────────────────────────────────────────────────────────────
export type KycStatus =
  | "not_started"
  | "pending"
  | "in_review"
  | "approved"
  | "rejected"
  | "expired";

export interface KycSession {
  id: string;
  userId: string;
  status: KycStatus;
  provider: "sumsub" | "jumio";
  submittedAt?: string;
  reviewedAt?: string;
  rejectionReason?: string;
  documents: KycDocument[];
}

export interface KycDocument {
  id: string;
  type: "passport" | "national_id" | "drivers_license" | "proof_of_address";
  status: "uploaded" | "reviewing" | "accepted" | "rejected";
  uploadedAt: string;
}

// ── Wallet ────────────────────────────────────────────────────────────────────
export interface Wallet {
  id: string;
  userId: string;
  address: string;       // on-chain EVM address
  balanceWei: string;    // GOLD token balance in wei (18 decimals)
  balanceGrams: string;  // human-readable grams
  custodial: boolean;
}

// ── Price ─────────────────────────────────────────────────────────────────────
export interface GoldPrice {
  pricePerGramTRY: string;
  pricePerGramUSD: string;
  pricePerOzUSD: string;
  source: string;
  updatedAt: string;
}

// ── Orders ───────────────────────────────────────────────────────────────────
export type OrderType = "buy" | "sell" | "redeem_cash" | "redeem_physical";

export type OrderStatus =
  | "CREATED"
  | "PAYMENT_PENDING"
  | "PAID"
  | "RESERVING_BARS"
  | "MINT_PROPOSED"
  | "MINT_EXECUTED"
  | "COMPLETED"
  | "CANCELLED"
  | "FAILED_NO_STOCK";

export interface Order {
  id: string;
  userId: string;
  type: OrderType;
  status: OrderStatus;
  amountGrams: string;  // e.g. "10.5"
  amountWei: string;    // token amount in wei
  pricePerGramTRY: string;
  totalTRY: string;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

// ── PoR ──────────────────────────────────────────────────────────────────────
export interface ProofOfReserve {
  id: string;
  cycleId: string;
  attestedAt: string;
  totalGoldGrams: string;
  totalTokenSupplyWei: string;
  totalTokenSupplyGrams: string;
  coverageRatio: string; // should be >= 1.0
  merkleRoot: string;
  ipfsCid: string;
  onChainTxHash: string;
  auditor: string;
  vaults: VaultSnapshot[];
}

export interface VaultSnapshot {
  vaultId: string;
  vaultName: string;
  location: string;
  totalGoldGrams: string;
  barCount: number;
  lastInspected: string;
}

// ── API response envelope ────────────────────────────────────────────────────
export interface ApiResponse<T> {
  data: T;
  meta: { request_id: string };
}

export interface ApiError {
  error: {
    code: string;   // GOLD.DOMAIN.001
    message: string;
    details?: Record<string, unknown>;
  };
  meta: { request_id: string };
}

export interface PaginatedResponse<T> {
  data: T[];
  meta: {
    request_id: string;
    cursor?: string;
    hasMore: boolean;
    total?: number;
  };
}

// ── Admin ─────────────────────────────────────────────────────────────────────

export interface AdminUserRow {
  id: string;
  email: string;
  fullName: string;
  arena: Arena;
  role: UserRole;
  kycStatus: KycStatus;
  kycSessionId?: string;
  balanceGrams: string;
  totalOrderCount: number;
  createdAt: string;
  lastActiveAt?: string;
}

export interface AdminKycReviewPayload {
  action: "approve" | "reject";
  reason?: string;
}

export type TokenOpType = "mint" | "burn" | "transfer";

export interface TokenOperation {
  id: string;
  type: TokenOpType;
  initiatedBy: string;       // admin user id
  targetUserId?: string;
  fromAddress?: string;
  toAddress?: string;
  amountGrams: string;
  amountWei: string;
  txHash?: string;
  status: "pending" | "submitted" | "confirmed" | "failed";
  createdAt: string;
  confirmedAt?: string;
}

export interface AdminTokenOpPayload {
  type: TokenOpType;
  targetUserId?: string;
  toAddress?: string;
  amountGrams: string;
  reason: string;
}

export interface AdminStats {
  totalUsers: number;
  kycPendingCount: number;
  kycApprovedCount: number;
  kycRejectedCount: number;
  totalTokenSupplyGrams: string;
  totalGoldReserveGrams: string;
  coverageRatio: string;
  totalOrderVolumeTRY: string;
  activeUsersLast30d: number;
  mintTotalGrams: string;
  burnTotalGrams: string;
}

// ── Compliance ────────────────────────────────────────────────────────────────

export type SanctionsListSource = "OFAC" | "EU" | "UN" | "FATF" | "TR_MASAK";

export type ComplianceFlagReason =
  | "sanctions_hit"
  | "pep_match"
  | "adverse_media"
  | "high_risk_jurisdiction"
  | "unusual_activity"
  | "structuring_detected";

export type ComplianceFlagStatus = "open" | "investigating" | "cleared" | "escalated";

export interface SanctionsScreeningResult {
  id: string;
  userId: string;
  userFullName: string;
  userEmail: string;
  screenedAt: string;
  listSource: SanctionsListSource;
  matchScore: number;      // 0-100
  matchedName: string;
  status: "no_match" | "potential_match" | "confirmed_hit";
  resolvedAt?: string;
  resolvedBy?: string;
}

export interface ComplianceFlag {
  id: string;
  userId: string;
  userFullName: string;
  userEmail: string;
  arena: Arena;
  reason: ComplianceFlagReason;
  status: ComplianceFlagStatus;
  description: string;
  riskScore: number;       // 1-10
  createdAt: string;
  updatedAt: string;
  assignedTo?: string;
}

// ── System Health ─────────────────────────────────────────────────────────────

export type ServiceStatusLevel = "operational" | "degraded" | "outage" | "maintenance";

export interface ServiceHealth {
  id: string;
  name: string;
  description: string;
  status: ServiceStatusLevel;
  latencyMs?: number;
  uptimePct?: number;       // last 30d
  lastCheckedAt: string;
  lastIncidentAt?: string;
  incidentNote?: string;
}

export interface SystemHealthSummary {
  overall: ServiceStatusLevel;
  services: ServiceHealth[];
  checkedAt: string;
}

// ── Fee Configuration ─────────────────────────────────────────────────────────

export type FeeArena = Arena | "global";

export interface FeeRule {
  id: string;
  arena: FeeArena;
  orderType: "buy" | "sell" | "redeem_cash" | "redeem_physical";
  feeBps: number;           // basis points, e.g. 50 = 0.5%
  minFeeTRY?: string;
  maxFeeTRY?: string;
  effectiveFrom: string;
  updatedAt: string;
  updatedBy: string;
}

export interface FeeConfigUpdatePayload {
  arena: FeeArena;
  orderType: FeeRule["orderType"];
  feeBps: number;
  minFeeTRY?: string;
  maxFeeTRY?: string;
}

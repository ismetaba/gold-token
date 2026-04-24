package http

import (
	"net/http"
	"strings"

	"github.com/ismetaba/gold-token/backend/services/admin/internal/domain"
)

// routePermissions maps route prefixes to the minimum roles that can access them,
// split by whether the request is read-only (GET/HEAD) or mutating.
//
// Permission matrix:
//
//	Route group         super_admin  ops  compliance_viewer  viewer
//	/admin/kyc/*           rw        --       r-only         r-only*
//	/admin/orders/*        rw        rw         --           r-only*
//	/admin/treasury/*      rw        rw         --           r-only*
//	/admin/vault/*         rw        rw         --           r-only*
//	/admin/fees/*          rw        rw         --           r-only*
//	/admin/audit/*         rw        --       r-only         r-only*
//	/admin/users/*         rw        --         --           r-only*
//	/admin/compliance/*    rw        --       r-only         r-only*
//
// (*) viewer has read-only access everywhere, all others listed are explicit.
type routeRule struct {
	// writeRoles can perform any HTTP method.
	writeRoles []domain.Role
	// readRoles can only perform safe methods (GET, HEAD, OPTIONS).
	readRoles []domain.Role
}

var permissionMap = map[string]routeRule{
	"/admin/kyc": {
		writeRoles: []domain.Role{domain.RoleSuperAdmin},
		readRoles:  []domain.Role{domain.RoleComplianceViewer, domain.RoleViewer},
	},
	"/admin/orders": {
		writeRoles: []domain.Role{domain.RoleSuperAdmin, domain.RoleOps},
		readRoles:  []domain.Role{domain.RoleViewer},
	},
	"/admin/treasury": {
		writeRoles: []domain.Role{domain.RoleSuperAdmin, domain.RoleOps},
		readRoles:  []domain.Role{domain.RoleViewer},
	},
	"/admin/vault": {
		writeRoles: []domain.Role{domain.RoleSuperAdmin, domain.RoleOps},
		readRoles:  []domain.Role{domain.RoleViewer},
	},
	"/admin/fees": {
		writeRoles: []domain.Role{domain.RoleSuperAdmin, domain.RoleOps},
		readRoles:  []domain.Role{domain.RoleViewer},
	},
	"/admin/audit": {
		writeRoles: []domain.Role{domain.RoleSuperAdmin},
		readRoles:  []domain.Role{domain.RoleComplianceViewer, domain.RoleViewer},
	},
	"/admin/users": {
		writeRoles: []domain.Role{domain.RoleSuperAdmin},
		readRoles:  []domain.Role{domain.RoleViewer},
	},
	"/admin/compliance": {
		writeRoles: []domain.Role{domain.RoleSuperAdmin},
		readRoles:  []domain.Role{domain.RoleComplianceViewer, domain.RoleViewer},
	},
}

// rbacMiddleware enforces role-based access on proxied route groups.
func rbacMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := claimsFromCtx(r.Context())
		role := domain.Role(claims.Role)

		// super_admin bypasses all checks.
		if role == domain.RoleSuperAdmin {
			next.ServeHTTP(w, r)
			return
		}

		// Identify which route group this request belongs to.
		rule, ok := matchRouteRule(r.URL.Path)
		if !ok {
			writeError(w, http.StatusForbidden, "forbidden", "no permission rule for route")
			return
		}

		isSafeMethod := isSafe(r.Method)

		if hasRole(role, rule.writeRoles) {
			next.ServeHTTP(w, r)
			return
		}

		if isSafeMethod && hasRole(role, rule.readRoles) {
			next.ServeHTTP(w, r)
			return
		}

		writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
	})
}

// requireRole is a middleware factory that restricts access to the given roles only.
func requireRole(allowed ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFromCtx(r.Context())
			if hasRole(domain.Role(claims.Role), allowed) {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		})
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func matchRouteRule(path string) (routeRule, bool) {
	// Match longest prefix first.
	best := ""
	var bestRule routeRule
	for prefix, rule := range permissionMap {
		if strings.HasPrefix(path, prefix+"/") || path == prefix {
			if len(prefix) > len(best) {
				best = prefix
				bestRule = rule
			}
		}
	}
	if best == "" {
		return routeRule{}, false
	}
	return bestRule, true
}

func hasRole(role domain.Role, allowed []domain.Role) bool {
	for _, a := range allowed {
		if a == role {
			return true
		}
	}
	return false
}

func isSafe(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

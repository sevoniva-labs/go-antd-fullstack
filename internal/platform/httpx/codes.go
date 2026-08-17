package httpx

// Enterprise response codes are stable protocol identifiers. HTTP status
// remains authoritative for transport semantics; these codes remain stable
// across translations and message wording changes.
//
// Ranges:
//
//	000000 success
//	10xxxx request/validation
//	20xxxx identity/security
//	30xxxx conflict/state
//	40xxxx dependency/infrastructure
//	50xxxx business/domain (reserved for projects)
//	90xxxx internal/platform
var codeMap = map[string]string{
	"SUCCESS":                       "000000",
	"INVALID_REQUEST":               "100001",
	"INVALID_JSON":                  "100002",
	"MISSING_FIELDS":                "100003",
	"NOT_FOUND":                     "100004",
	"INVALID_LOGIN_NAME":            "100005",
	"INVALID_ORGANIZATION":          "100006",
	"INVALID_SECURITY_CONFIG":       "100007",
	"INVALID_ARGUMENT":              "100008",
	"INVALID_EXPORT_FORMAT":         "100009",
	"INVALID_EXPORT_LIMIT":          "100010",
	"UNAUTHENTICATED":               "200001",
	"INVALID_CREDENTIALS":           "200002",
	"LOGIN_FAILED":                  "200002",
	"ACCOUNT_LOCKED":                "200003",
	"PERMISSION_DENIED":             "200004",
	"CSRF_MISMATCH":                 "200005",
	"RATE_LIMITED":                  "200006",
	"PASSWORD_CHANGE_REQUIRED":      "200007",
	"PASSWORD_POLICY_VIOLATION":     "200008",
	"PASSWORD_CHANGE_FAILED":        "200009",
	"CURRENT_PASSWORD_INVALID":      "200010",
	"INVALID_ROLE":                  "200011",
	"PASSWORD_REUSED":               "200012",
	"ROLE_PERMISSION_UPDATE_FAILED": "200013",
	"USER_ROLE_UPDATE_FAILED":       "200014",
	"SESSION_REVOKE_FAILED":         "200015",
	"USER_STATUS_UPDATE_FAILED":     "200016",
	"USER_UNLOCK_FAILED":            "200017",
	"PASSWORD_RESET_FAILED":         "200018",
	"LAST_SYSTEM_ADMIN":             "200019",
	"INTERACTIVE_SESSION_REQUIRED":  "200020",
	"GRANT_CEILING_EXCEEDED":        "200021",
	"LOGOUT_FAILED":                 "200022",
	"LOGIN_NAME_CONFLICT":           "300001",
	"CONFLICT":                      "300002",
	"PASSWORD_STATE_CHANGED":        "300003",
	"DEPENDENCY_UNAVAILABLE":        "400001",
	"STORAGE_UNAVAILABLE":           "400002",
	"RELIABLE_AUDIT_UNAVAILABLE":    "400003",
	"RATE_LIMIT_UNAVAILABLE":        "400004",
	"INTERNAL":                      "900000",
	"CREATE_USER_FAILED":            "900001",
}

func NumericCode(symbol string) string {
	if v, ok := codeMap[symbol]; ok {
		return v
	}
	return "900099"
}

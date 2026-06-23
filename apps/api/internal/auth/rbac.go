package auth

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/portierglobal/hijau/apps/api/internal/db"
)

// Perm is a fine-grained permission. Token scopes are expressed using these
// same strings, so a PAT scope of "translations:write" maps 1:1.
type Perm string

const (
	PermProjectRead       Perm = "project:read"
	PermProjectWrite      Perm = "project:write"  // languages, namespaces, project settings
	PermProjectAdmin      Perm = "project:admin"  // members, API keys, webhooks, delete project
	PermKeysWrite         Perm = "keys:write"
	PermTranslationsRead  Perm = "translations:read"
	PermTranslationsWrite Perm = "translations:write"
	PermReview            Perm = "translations:review" // approve / mark needs-work
	PermComment           Perm = "comments:write"
	PermScreenshotWrite   Perm = "screenshots:write"
	PermImportExport      Perm = "importexport:run"
	PermMTUse             Perm = "mt:use"
)

// AllPerms is the full set granted to owners/admins.
var AllPerms = []Perm{
	PermProjectRead, PermProjectWrite, PermProjectAdmin, PermKeysWrite,
	PermTranslationsRead, PermTranslationsWrite, PermReview, PermComment,
	PermScreenshotWrite, PermImportExport, PermMTUse,
}

// rolePerms is the permission grid keyed by project role.
var rolePerms = map[db.ProjectRole][]Perm{
	db.ProjectRoleOwner: AllPerms,
	db.ProjectRoleAdmin: AllPerms,
	db.ProjectRoleDeveloper: {
		PermProjectRead, PermProjectWrite, PermKeysWrite,
		PermTranslationsRead, PermTranslationsWrite,
		PermComment, PermScreenshotWrite, PermImportExport, PermMTUse,
	},
	db.ProjectRoleReviewer: {
		PermProjectRead, PermTranslationsRead, PermTranslationsWrite,
		PermReview, PermComment, PermScreenshotWrite,
	},
	db.ProjectRoleTranslator: {
		PermProjectRead, PermTranslationsRead, PermTranslationsWrite, PermComment,
	},
}

// isLanguageScoped reports whether a permission is gated by per-language access
// for translator/reviewer roles.
func isLanguageScoped(p Perm) bool { return p == PermTranslationsWrite || p == PermReview }

func scopedRole(r db.ProjectRole) bool {
	return r == db.ProjectRoleTranslator || r == db.ProjectRoleReviewer
}

// ErrForbidden is returned when the principal may not perform the action.
var ErrForbidden = errors.New("auth: forbidden")

// AuthzQuerier is the subset of db.Queries the authorizer needs.
type AuthzQuerier interface {
	GetProjectForAuth(ctx context.Context, id string) (db.GetProjectForAuthRow, error)
	GetProjectMember(ctx context.Context, arg db.GetProjectMemberParams) (db.ProjectMember, error)
	GetOrgMembership(ctx context.Context, arg db.GetOrgMembershipParams) (db.OrgMembership, error)
	ListProjectMemberLanguageIDs(ctx context.Context, memberID string) ([]string, error)
}

// Check carries the resource context for an authorization decision.
type Check struct {
	ProjectID  string // empty for non-project-scoped actions
	LanguageID string // required for language-scoped perms on scoped roles
}

// Authorize returns nil if the principal in ctx may perform perm under check,
// otherwise ErrForbidden (or a database error).
func Authorize(ctx context.Context, q AuthzQuerier, perm Perm, check Check) error {
	p := FromContext(ctx)
	if !p.IsAuthenticated() {
		return ErrForbidden
	}

	// Explicit token scopes narrow whatever the underlying role would allow.
	if len(p.Scopes) > 0 && !slices.Contains(p.Scopes, string(perm)) {
		return ErrForbidden
	}

	switch p.Kind {
	case APIKeyPrincipal:
		if p.APIKeyType == db.ApiKeyTypeProject || p.APIKeyType == db.ApiKeyTypeEditor {
			// Project/editor keys derive authority purely from their scopes,
			// constrained to their single project. No scopes => can't act.
			if check.ProjectID == "" || check.ProjectID != p.ProjectID || len(p.Scopes) == 0 {
				return ErrForbidden
			}
			return nil // perm already verified to be in scopes above
		}
		// PAT: authority comes from the owner's project role (then scope-narrowed).
		return authorizeUser(ctx, q, p.UserID, perm, check)
	case UserPrincipal:
		return authorizeUser(ctx, q, p.UserID, perm, check)
	default:
		return ErrForbidden
	}
}

func authorizeUser(ctx context.Context, q AuthzQuerier, userID string, perm Perm, check Check) error {
	if check.ProjectID == "" {
		return nil // non-project action; being authenticated is sufficient
	}
	role, langs, err := resolveProjectRole(ctx, q, userID, check.ProjectID)
	if err != nil {
		return err
	}
	if !slices.Contains(rolePerms[role], perm) {
		return ErrForbidden
	}
	if scopedRole(role) && isLanguageScoped(perm) && len(langs) > 0 {
		if check.LanguageID == "" || !slices.Contains(langs, check.LanguageID) {
			return ErrForbidden
		}
	}
	return nil
}

// resolveProjectRole returns the user's effective role in a project: their
// direct membership, or admin via org ownership/admin. The returned language
// list is non-empty only for scoped roles that have explicit language grants.
func resolveProjectRole(ctx context.Context, q AuthzQuerier, userID, projectID string) (db.ProjectRole, []string, error) {
	m, err := q.GetProjectMember(ctx, db.GetProjectMemberParams{ProjectID: projectID, UserID: userID})
	switch {
	case err == nil:
		var langs []string
		if scopedRole(m.Role) {
			if langs, err = q.ListProjectMemberLanguageIDs(ctx, m.ID); err != nil {
				return "", nil, err
			}
		}
		return m.Role, langs, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return "", nil, err
	}

	// Not a direct member — allow org owners/admins as project admins.
	proj, err := q.GetProjectForAuth(ctx, projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, ErrForbidden
		}
		return "", nil, err
	}
	om, err := q.GetOrgMembership(ctx, db.GetOrgMembershipParams{OrgID: proj.OrgID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, ErrForbidden
		}
		return "", nil, err
	}
	if om.Role == db.OrgRoleOwner || om.Role == db.OrgRoleAdmin {
		return db.ProjectRoleAdmin, nil, nil
	}
	return "", nil, ErrForbidden
}

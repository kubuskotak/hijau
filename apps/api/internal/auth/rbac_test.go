package auth

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/portierglobal/hijau/apps/api/internal/db"
)

type fakeQ struct {
	members map[string]db.ProjectMember       // "projectID:userID"
	orgs    map[string]db.OrgMembership       // "orgID:userID"
	langs   map[string][]string               // memberID -> language ids
	proj    map[string]db.GetProjectForAuthRow // projectID -> row
}

func (f fakeQ) GetProjectMember(_ context.Context, a db.GetProjectMemberParams) (db.ProjectMember, error) {
	if m, ok := f.members[a.ProjectID+":"+a.UserID]; ok {
		return m, nil
	}
	return db.ProjectMember{}, pgx.ErrNoRows
}

func (f fakeQ) GetOrgMembership(_ context.Context, a db.GetOrgMembershipParams) (db.OrgMembership, error) {
	if m, ok := f.orgs[a.OrgID+":"+a.UserID]; ok {
		return m, nil
	}
	return db.OrgMembership{}, pgx.ErrNoRows
}

func (f fakeQ) ListProjectMemberLanguageIDs(_ context.Context, memberID string) ([]string, error) {
	return f.langs[memberID], nil
}

func (f fakeQ) GetProjectForAuth(_ context.Context, id string) (db.GetProjectForAuthRow, error) {
	if r, ok := f.proj[id]; ok {
		return r, nil
	}
	return db.GetProjectForAuthRow{}, pgx.ErrNoRows
}

func newFakeQ() fakeQ {
	return fakeQ{
		members: map[string]db.ProjectMember{
			"p1:u_owner": {ID: "mem-owner", ProjectID: "p1", UserID: "u_owner", Role: db.ProjectRoleOwner},
			"p1:u_dev":   {ID: "mem-dev", ProjectID: "p1", UserID: "u_dev", Role: db.ProjectRoleDeveloper},
			"p1:u_tr":    {ID: "mem-tr", ProjectID: "p1", UserID: "u_tr", Role: db.ProjectRoleTranslator},
			"p1:u_rv":    {ID: "mem-rv", ProjectID: "p1", UserID: "u_rv", Role: db.ProjectRoleReviewer},
		},
		orgs:  map[string]db.OrgMembership{"o1:u_orgadmin": {OrgID: "o1", UserID: "u_orgadmin", Role: db.OrgRoleAdmin}},
		langs: map[string][]string{"mem-tr": {"fr"}, "mem-rv": {"de"}},
		proj:  map[string]db.GetProjectForAuthRow{"p1": {ID: "p1", OrgID: "o1"}},
	}
}

func user(id string) Principal { return Principal{Kind: UserPrincipal, UserID: id} }
func pat(id string, scopes ...string) Principal {
	return Principal{Kind: APIKeyPrincipal, APIKeyType: db.ApiKeyTypePat, UserID: id, Scopes: scopes}
}
func projKey(project string, scopes ...string) Principal {
	return Principal{Kind: APIKeyPrincipal, APIKeyType: db.ApiKeyTypeProject, ProjectID: project, Scopes: scopes}
}
func editorKey(project string, scopes ...string) Principal {
	return Principal{Kind: APIKeyPrincipal, APIKeyType: db.ApiKeyTypeEditor, ProjectID: project, Scopes: scopes}
}

func TestAuthorize(t *testing.T) {
	q := newFakeQ()
	cases := []struct {
		name      string
		principal Principal
		perm      Perm
		check     Check
		wantOK    bool
	}{
		{"anonymous denied", Principal{Kind: Anonymous}, PermProjectRead, Check{ProjectID: "p1"}, false},
		{"owner can admin", user("u_owner"), PermProjectAdmin, Check{ProjectID: "p1"}, true},
		{"developer can write keys", user("u_dev"), PermKeysWrite, Check{ProjectID: "p1"}, true},
		{"developer cannot admin", user("u_dev"), PermProjectAdmin, Check{ProjectID: "p1"}, false},
		{"translator writes scoped lang", user("u_tr"), PermTranslationsWrite, Check{ProjectID: "p1", LanguageID: "fr"}, true},
		{"translator denied other lang", user("u_tr"), PermTranslationsWrite, Check{ProjectID: "p1", LanguageID: "de"}, false},
		{"translator cannot review", user("u_tr"), PermReview, Check{ProjectID: "p1", LanguageID: "fr"}, false},
		{"reviewer reviews scoped lang", user("u_rv"), PermReview, Check{ProjectID: "p1", LanguageID: "de"}, true},
		{"reviewer denied other lang", user("u_rv"), PermReview, Check{ProjectID: "p1", LanguageID: "fr"}, false},
		{"pat inherits owner (no scopes)", pat("u_owner"), PermProjectAdmin, Check{ProjectID: "p1"}, true},
		{"pat scope narrows write", pat("u_owner", "translations:read"), PermTranslationsWrite, Check{ProjectID: "p1"}, false},
		{"pat scope allows read", pat("u_owner", "translations:read"), PermTranslationsRead, Check{ProjectID: "p1"}, true},
		{"project key writes own project", projKey("p1", "translations:write"), PermTranslationsWrite, Check{ProjectID: "p1"}, true},
		{"project key denied other project", projKey("p1", "translations:write"), PermTranslationsWrite, Check{ProjectID: "p2"}, false},
		{"editor key no scopes denied", editorKey("p1"), PermTranslationsRead, Check{ProjectID: "p1"}, false},
		{"editor key read scope ok", editorKey("p1", "translations:read"), PermTranslationsRead, Check{ProjectID: "p1"}, true},
		{"org admin via fallback", user("u_orgadmin"), PermProjectAdmin, Check{ProjectID: "p1"}, true},
		{"stranger denied", user("u_stranger"), PermProjectRead, Check{ProjectID: "p1"}, false},
		{"non-project action ok", user("u_stranger"), PermProjectRead, Check{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := WithPrincipal(context.Background(), c.principal)
			err := Authorize(ctx, q, c.perm, c.check)
			if c.wantOK && err != nil {
				t.Fatalf("want allowed, got %v", err)
			}
			if !c.wantOK && err == nil {
				t.Fatal("want denied, got allowed")
			}
		})
	}
}

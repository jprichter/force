package lib

import (
	"errors"
	"testing"
)

const (
	lazyTestOrgId  = "00D123456789012AAA"
	lazyTestUser   = "scratch@example.com"
	lazyTestUserId = "005123456789012AAA"
)

// lazySFDXSession mimics what SFDXAuthToForceSession builds from an auth
// file that has no userId.
func lazySFDXSession() *ForceSession {
	return &ForceSession{
		AccessToken: "token",
		InstanceUrl: "https://example.my.salesforce.com",
		Id:          lazyTestOrgId,
		UserInfo:    &UserInfo{UserName: lazyTestUser, OrgId: lazyTestOrgId},
	}
}

// countUserInfoFetches stubs getUserInfoFn and counts how often it is called.
func countUserInfoFetches(t *testing.T, info UserInfo, err error) *int {
	t.Helper()
	calls := 0
	original := getUserInfoFn
	getUserInfoFn = func(_ *ForceSession) (UserInfo, error) {
		calls++
		return info, err
	}
	t.Cleanup(func() { getUserInfoFn = original })
	return &calls
}

func fullUserInfo() UserInfo {
	return UserInfo{
		UserName:     lazyTestUser,
		OrgId:        lazyTestOrgId,
		UserId:       lazyTestUserId,
		ProfileId:    "00e123456789012AAA",
		OrgNamespace: "ns",
	}
}

func TestCompleteSFDXSessionDoesNotFetchUserInfo(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := lazySFDXSession()

	if err := CompleteSFDXSession(creds, "scratch"); err != nil {
		t.Fatalf("CompleteSFDXSession: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("expected no user info fetch, got %d", *calls)
	}
	if creds.UserInfo.UserName != lazyTestUser || creds.UserInfo.OrgId != lazyTestOrgId {
		t.Fatalf("known user info lost: %+v", creds.UserInfo)
	}
	if creds.SessionOptions.Alias != "scratch" {
		t.Fatalf("alias = %q, want scratch", creds.SessionOptions.Alias)
	}
}

func TestCompleteSFDXSessionFetchesWhenUserIsUnknown(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := &ForceSession{AccessToken: "token", InstanceUrl: "https://example.my.salesforce.com"}

	if err := CompleteSFDXSession(creds, ""); err != nil {
		t.Fatalf("CompleteSFDXSession: %v", err)
	}
	if *calls != 1 || creds.UserInfo == nil || creds.UserInfo.UserName != lazyTestUser {
		t.Fatalf("expected one fetch filling the user name, got %d fetches and %+v", *calls, creds.UserInfo)
	}
	if creds.Id != lazyTestOrgId {
		t.Fatalf("org id = %q, want %s", creds.Id, lazyTestOrgId)
	}
}

func TestEnsureSessionUserInfoFetchesOnce(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := lazySFDXSession()

	for i := 0; i < 2; i++ {
		if err := EnsureSessionUserInfo(creds); err != nil {
			t.Fatalf("EnsureSessionUserInfo: %v", err)
		}
	}
	if *calls != 1 {
		t.Fatalf("expected 1 fetch, got %d", *calls)
	}
	if creds.UserInfo.UserId != lazyTestUserId || creds.UserInfo.OrgNamespace != "ns" {
		t.Fatalf("user info not populated: %+v", creds.UserInfo)
	}
}

func TestEnsureSessionUserInfoKeepsKnownFieldsWhenResponseLacksThem(t *testing.T) {
	info := fullUserInfo()
	info.UserName, info.OrgId = "", ""
	countUserInfoFetches(t, info, nil)
	creds := lazySFDXSession()

	if err := EnsureSessionUserInfo(creds); err != nil {
		t.Fatalf("EnsureSessionUserInfo: %v", err)
	}
	if creds.UserInfo.UserName != lazyTestUser || creds.UserInfo.OrgId != lazyTestOrgId {
		t.Fatalf("known fields overwritten: %+v", creds.UserInfo)
	}
}

func TestEnsureSessionUserInfoSkipsCompleteSessions(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	creds := lazySFDXSession()
	creds.UserInfo.UserId = lazyTestUserId

	if err := EnsureSessionUserInfo(creds); err != nil {
		t.Fatalf("EnsureSessionUserInfo: %v", err)
	}
	if *calls != 0 {
		t.Fatalf("expected no fetch for a session with a user id, got %d", *calls)
	}
}

func TestEnsureSessionUserInfoReturnsFetchErrors(t *testing.T) {
	countUserInfoFetches(t, UserInfo{}, errors.New("boom"))
	creds := lazySFDXSession()

	if err := EnsureSessionUserInfo(creds); err == nil {
		t.Fatal("expected an error")
	}
	if creds.UserInfo.UserName != lazyTestUser {
		t.Fatalf("session modified on error: %+v", creds.UserInfo)
	}
}

func TestForceEnsureUserInfoLoadsUserInfo(t *testing.T) {
	calls := countUserInfoFetches(t, fullUserInfo(), nil)
	f := NewForce(lazySFDXSession())

	if err := f.EnsureUserInfo(); err != nil {
		t.Fatalf("EnsureUserInfo: %v", err)
	}
	if *calls != 1 || f.Credentials.UserInfo.ProfileId == "" {
		t.Fatalf("expected the Force helper to load user info, got %d fetches and %+v", *calls, f.Credentials.UserInfo)
	}
}

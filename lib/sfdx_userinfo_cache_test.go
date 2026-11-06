package lib

import (
	"testing"
	"time"

	forceConfig "github.com/ForceCLI/force/config"
)

const (
	cacheTestOrgId = "00D123456789012AAA"
	cacheTestUser  = "Scratch@Example.com"
	cacheTestKey   = cacheTestOrgId + "_scratch@example.com"
)

// sfdxSession mimics what SFDXAuthToForceSession builds from an auth file
// that has no userId.
func sfdxSession() *ForceSession {
	return &ForceSession{
		AccessToken: "token",
		InstanceUrl: "https://example.my.salesforce.com",
		Id:          cacheTestOrgId,
		UserInfo:    &UserInfo{UserName: cacheTestUser, OrgId: cacheTestOrgId},
	}
}

// countingUserInfo stubs getUserInfoFn and counts how often it is called.
func countingUserInfo(t *testing.T) *int {
	t.Helper()
	calls := 0
	original := getUserInfoFn
	getUserInfoFn = func(_ *ForceSession) (UserInfo, error) {
		calls++
		return UserInfo{
			UserName:     cacheTestUser,
			OrgId:        cacheTestOrgId,
			UserId:       "005123456789012AAA",
			ProfileId:    "00e123456789012AAA",
			OrgNamespace: "",
		}, nil
	}
	t.Cleanup(func() { getUserInfoFn = original })
	return &calls
}

func fixNow(t *testing.T, at time.Time) {
	t.Helper()
	original := nowFn
	nowFn = func() time.Time { return at }
	t.Cleanup(func() { nowFn = original })
}

func completeOrFail(t *testing.T) *ForceSession {
	t.Helper()
	creds := sfdxSession()
	if err := CompleteSFDXSession(creds, "scratch"); err != nil {
		t.Fatalf("CompleteSFDXSession: %v", err)
	}
	if creds.UserInfo == nil || creds.UserInfo.UserId != "005123456789012AAA" {
		t.Fatalf("UserInfo not populated: %+v", creds.UserInfo)
	}
	return creds
}

func TestCompleteSFDXSessionCachesUserInfo(t *testing.T) {
	defer setupTestConfig(t)()
	calls := countingUserInfo(t)

	completeOrFail(t)
	completeOrFail(t)

	if *calls != 1 {
		t.Fatalf("expected 1 user info fetch, got %d", *calls)
	}
	if _, err := forceConfig.Config.LoadGlobal("userinfo", cacheTestKey); err != nil {
		t.Fatalf("cache entry not written under %s: %v", cacheTestKey, err)
	}
}

func TestCompleteSFDXSessionRefetchesExpiredUserInfo(t *testing.T) {
	defer setupTestConfig(t)()
	calls := countingUserInfo(t)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	fixNow(t, start)
	completeOrFail(t)
	fixNow(t, start.Add(userInfoCacheTTL+time.Minute))
	completeOrFail(t)

	if *calls != 2 {
		t.Fatalf("expected a refetch after the TTL, got %d fetches", *calls)
	}
}

func TestCompleteSFDXSessionIgnoresCorruptCache(t *testing.T) {
	defer setupTestConfig(t)()
	calls := countingUserInfo(t)
	if err := forceConfig.Config.SaveGlobal("userinfo", cacheTestKey, "{not json"); err != nil {
		t.Fatal(err)
	}

	completeOrFail(t)
	completeOrFail(t)

	if *calls != 1 {
		t.Fatalf("expected the corrupt entry to be replaced by one fetch, got %d", *calls)
	}
}

func TestCompleteSFDXSessionCacheCanBeDisabled(t *testing.T) {
	defer setupTestConfig(t)()
	t.Setenv("FORCE_NO_USERINFO_CACHE", "1")
	calls := countingUserInfo(t)

	completeOrFail(t)
	completeOrFail(t)

	if *calls != 2 {
		t.Fatalf("expected a fetch per call with the cache disabled, got %d", *calls)
	}
	if _, err := forceConfig.Config.LoadGlobal("userinfo", cacheTestKey); err == nil {
		t.Fatalf("cache entry written although the cache is disabled")
	}
}

func TestClearCachedUserInfoRemovesEntry(t *testing.T) {
	defer setupTestConfig(t)()
	calls := countingUserInfo(t)

	creds := completeOrFail(t)
	ClearCachedUserInfo(creds)
	completeOrFail(t)

	if *calls != 2 {
		t.Fatalf("expected a refetch after clearing the cache, got %d fetches", *calls)
	}
}

package service

import (
	"regexp"
	"testing"
)

func TestUUIDAndAuthTokenRemainWireCompatible(t *testing.T) {
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	identifier := UUID()
	if !uuidPattern.MatchString(identifier) {
		t.Fatalf("UUID() = %q", identifier)
	}
	auth := NewAuthService(func() string { return "unused" })
	profile, err := auth.Login(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Token == "" || profile.UserID != "ramanred" || profile.Role != "SRE_LEAD" {
		t.Fatalf("unexpected auth profile: %+v", profile)
	}
	claims, err := auth.VerifyToken(profile.Token)
	if err != nil || claims.UserID != "ramanred" {
		t.Fatalf("VerifyToken() claims=%+v err=%v", claims, err)
	}
}

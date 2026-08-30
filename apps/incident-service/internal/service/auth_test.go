package service

import (
	"strings"
	"testing"
	"time"
)

func TestSecureAuthSeparatesPasswordSigningKeyAndServerRole(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	auth, err := NewConfiguredAuthService(strings.Repeat("s", 32), "correct horse battery", RoleDevOpsEngineer, false, func() time.Time { return now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	username, wrongPassword, requestedRole := "Operator", "wrong password", RoleSRELead
	if _, err := auth.Login(&username, &wrongPassword, &requestedRole); !IsCode(err, CodeInvalid) {
		t.Fatalf("wrong password error=%v", err)
	}
	password := "correct horse battery"
	profile, err := auth.Login(&username, &password, &requestedRole)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Role != RoleDevOpsEngineer {
		t.Fatalf("client escalated role to %q", profile.Role)
	}
	claims, err := auth.VerifyToken(profile.Token)
	if err != nil || claims.Role != RoleDevOpsEngineer || claims.UserID != "operator" {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
}

func TestSecureAuthRejectsTamperedAndExpiredSessions(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	auth, err := NewConfiguredAuthService(strings.Repeat("t", 32), "correct horse battery", RoleSRELead, false, clock, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	username, password := "Operator", "correct horse battery"
	profile, err := auth.Login(&username, &password, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.VerifyToken(profile.Token + "x"); err == nil {
		t.Fatal("tampered token was accepted")
	}
	now = now.Add(2 * time.Minute)
	if _, err := auth.VerifyToken(profile.Token); err == nil {
		t.Fatal("expired token was accepted")
	}
}

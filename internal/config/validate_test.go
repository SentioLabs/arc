package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sentiolabs/arc/internal/config"
)

func TestValidateAcceptsDefault(t *testing.T) {
	if err := config.Validate(config.Default()); err != nil {
		t.Fatalf("Validate(Default()) = %v, want nil", err)
	}
}

func TestValidateRejectsBadURL(t *testing.T) {
	cfg := config.Default()
	cfg.CLI.Server = "not a url"
	err := config.Validate(cfg)
	var ve config.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationError", err)
	}
	if _, ok := ve["cli.server"]; !ok {
		t.Errorf("missing cli.server in errors: %v", ve)
	}
}

func TestValidateRejectsBadPort(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Port = 0
	err := config.Validate(cfg)
	var ve config.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationError", err)
	}
	if _, ok := ve["server.port"]; !ok {
		t.Errorf("missing server.port in errors: %v", ve)
	}
}

func TestValidateRejectsBadChannel(t *testing.T) {
	cfg := config.Default()
	cfg.Updates.Channel = "weekly"
	err := config.Validate(cfg)
	var ve config.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationError", err)
	}
	if _, ok := ve["updates.channel"]; !ok {
		t.Errorf("missing updates.channel in errors: %v", ve)
	}
}

func TestValidateRejectsEmptyCLIServer(t *testing.T) {
	cfg := config.Default()
	cfg.CLI.Server = ""
	err := config.Validate(cfg)
	var ve config.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationError", err)
	}
	if _, ok := ve["cli.server"]; !ok {
		t.Errorf("missing cli.server in errors: %v", ve)
	}
}

func TestValidateRejectsEmptyShareServer(t *testing.T) {
	cfg := config.Default()
	cfg.Share.Server = ""
	err := config.Validate(cfg)
	var ve config.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationError", err)
	}
	if _, ok := ve["share.server"]; !ok {
		t.Errorf("missing share.server in errors: %v", ve)
	}
}

func TestValidatePlansDir(t *testing.T) {
	base := config.Default()
	base.Plans.Dir = ""
	if config.Validate(base) == nil {
		t.Fatal("empty dir should fail")
	}
	base.Plans.Dir = "../x"
	if config.Validate(base) == nil {
		t.Fatal(".. should fail")
	}
	base.Plans.Dir = "~/V/{nope}"
	if config.Validate(base) == nil {
		t.Fatal("unknown var should fail")
	}
	base.Plans.Dir = "~/V/{project}"
	if err := config.Validate(base); err != nil {
		t.Fatalf("valid dir should pass: %v", err)
	}
}

func TestValidatePlansDirFirstUnknownVarReported(t *testing.T) {
	cfg := config.Default()
	// Template with two unknown vars; the FIRST one ({foo}) should be reported.
	cfg.Plans.Dir = "~/{foo}/{bar}"
	err := config.Validate(cfg)
	var ve config.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationError", err)
	}
	msg, ok := ve["plans.dir"]
	if !ok {
		t.Fatalf("missing plans.dir in errors: %v", ve)
	}
	if !strings.Contains(msg, "foo") {
		t.Errorf("expected error to mention first unknown var 'foo', got: %s", msg)
	}
	if strings.Contains(msg, "bar") {
		t.Errorf("error should NOT mention second var 'bar' (first-only reporting), got: %s", msg)
	}
}

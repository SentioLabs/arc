package config

import (
	"errors"
	"testing"
)

func TestValidateAcceptsDefault(t *testing.T) {
	if err := Validate(Default()); err != nil {
		t.Fatalf("Validate(Default()) = %v, want nil", err)
	}
}

func TestValidateRejectsBadURL(t *testing.T) {
	cfg := Default()
	cfg.CLI.Server = "not a url"
	err := Validate(cfg)
	var ve ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationErrors", err)
	}
	if _, ok := ve["cli.server"]; !ok {
		t.Errorf("missing cli.server in errors: %v", ve)
	}
}

func TestValidateRejectsBadPort(t *testing.T) {
	cfg := Default()
	cfg.Server.Port = 0
	err := Validate(cfg)
	var ve ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationErrors", err)
	}
	if _, ok := ve["server.port"]; !ok {
		t.Errorf("missing server.port in errors: %v", ve)
	}
}

func TestValidateRejectsBadChannel(t *testing.T) {
	cfg := Default()
	cfg.Updates.Channel = "weekly"
	err := Validate(cfg)
	var ve ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationErrors", err)
	}
	if _, ok := ve["updates.channel"]; !ok {
		t.Errorf("missing updates.channel in errors: %v", ve)
	}
}

func TestValidateRejectsEmptyCLIServer(t *testing.T) {
	cfg := Default()
	cfg.CLI.Server = ""
	err := Validate(cfg)
	var ve ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationErrors", err)
	}
	if _, ok := ve["cli.server"]; !ok {
		t.Errorf("missing cli.server in errors: %v", ve)
	}
}

func TestValidateRejectsEmptyShareServer(t *testing.T) {
	cfg := Default()
	cfg.Share.Server = ""
	err := Validate(cfg)
	var ve ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("err type = %T, want ValidationErrors", err)
	}
	if _, ok := ve["share.server"]; !ok {
		t.Errorf("missing share.server in errors: %v", ve)
	}
}

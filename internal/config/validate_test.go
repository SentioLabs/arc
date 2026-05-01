package config_test

import (
	"errors"
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

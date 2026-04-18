package tasks

import (
	"reflect"
	"testing"

	"github.com/hiyamamo/ws-dev/internal/config"
)

func TestFields(t *testing.T) {
	cases := map[string][]string{
		"bundle exec rails console":       {"bundle", "exec", "rails", "console"},
		`bundle exec "rails s -p 3000"`:   {"bundle", "exec", "rails s -p 3000"},
		`echo 'hello world' foo`:          {"echo", "hello world", "foo"},
		"  leading   and   trailing  ":    {"leading", "and", "trailing"},
	}
	for in, want := range cases {
		got := fields(in)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("fields(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildArgvWithWrapper(t *testing.T) {
	cfg := &config.Config{
		ExecWrapper: []string{"direnv", "exec", "."},
		Tasks:       map[string]string{"console": "bundle exec rails console"},
	}
	got := BuildArgv(cfg, cfg.Tasks["console"], []string{"--sandbox"})
	want := []string{"direnv", "exec", ".", "bundle", "exec", "rails", "console", "--sandbox"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildArgvNoWrapper(t *testing.T) {
	cfg := &config.Config{
		Tasks: map[string]string{"hello": "echo hi"},
	}
	got := BuildArgv(cfg, cfg.Tasks["hello"], nil)
	want := []string{"echo", "hi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

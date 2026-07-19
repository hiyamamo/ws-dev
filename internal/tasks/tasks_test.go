package tasks

import (
	"reflect"
	"testing"

	"github.com/hiyamamo/ws-dev/internal/config"
)

func TestFields(t *testing.T) {
	cases := map[string][]string{
		"bundle exec rails console":     {"bundle", "exec", "rails", "console"},
		`bundle exec "rails s -p 3000"`: {"bundle", "exec", "rails s -p 3000"},
		`echo 'hello world' foo`:        {"echo", "hello world", "foo"},
		"  leading   and   trailing  ":  {"leading", "and", "trailing"},
	}
	for in, want := range cases {
		got := fields(in)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("fields(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildArgvWithWrapper(t *testing.T) {
	cfg := &config.RepoConfig{
		ExecWrapper: []string{"direnv", "exec", "."},
		Tasks:       map[string]string{"console": "bundle exec rails console"},
	}
	got := BuildArgv(cfg, cfg.Tasks["console"], []string{"--sandbox"})
	want := []string{"direnv", "exec", ".", "bundle", "exec", "rails", "console", "--sandbox"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpand(t *testing.T) {
	v := Vars{Worktree: "feature-x", Root: "/repo", Dir: "/repo/.claude/worktrees/feature-x", PortBase: 4000}
	got, err := Expand("rails db:create RAILS_ENV={{.Worktree}} PORT={{.PortBase}}", v)
	if err != nil {
		t.Fatal(err)
	}
	want := "rails db:create RAILS_ENV=feature-x PORT=4000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	if _, err := Expand("{{.Nope}}", v); err == nil {
		t.Error("unknown template var should error")
	}
}

func TestEnv(t *testing.T) {
	v := Vars{Worktree: "feature-x", Root: "/repo", Dir: "/wt", PortBase: 4000}
	got := Env(v, "/wt/log")
	want := []string{
		"WS_DEV_WORKTREE=feature-x",
		"WS_DEV_ROOT=/repo",
		"WS_DEV_DIR=/wt",
		"WS_DEV_PORT_BASE=4000",
		"WS_DEV_LOG_DIR=/wt/log",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildArgvNoWrapper(t *testing.T) {
	cfg := &config.RepoConfig{
		Tasks: map[string]string{"hello": "echo hi"},
	}
	got := BuildArgv(cfg, cfg.Tasks["hello"], nil)
	want := []string{"echo", "hi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

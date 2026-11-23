package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"depman/internal/model"
	"depman/internal/store"
)

func TestCreateEnvWithTags(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "t.db")
	s, err := store.Open(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	e, err := s.CreateEnvironment(model.EnvironmentInput{
		Name: "e1", Kind: "dev", Description: "备注", Tags: []string{"alpha", "beta"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Tags) != 2 {
		t.Fatalf("tags=%v", e.Tags)
	}

	// 回归：MaxOpenConns(1) 下 List 不可嵌套 Query，否则会死锁
	done := make(chan error, 1)
	go func() {
		list, err := s.ListEnvironments()
		if err != nil {
			done <- err
			return
		}
		if len(list) != 1 || len(list[0].Tags) != 2 {
			done <- errUnexpected
			return
		}
		done <- nil
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ListEnvironments deadlock (nested query with MaxOpenConns=1)")
	}
}

var errUnexpected = errString("unexpected list result")

type errString string

func (e errString) Error() string { return string(e) }

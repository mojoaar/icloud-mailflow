package db

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentReadWrite(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := NewRulesRepo(d)
	if err := repo.Create(&Rule{Name: "seed", Enabled: true}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 64)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				if _, err := repo.List(); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			if err := repo.Create(&Rule{Name: fmt.Sprintf("r%d", j), Enabled: true}); err != nil {
				errCh <- err
				return
			}
		}
	}()
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent access error: %v", err)
	}
}

package activework

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var ErrOwned = errors.New("repository already has an active execution owner")

// Owner serializes all mutations for one repository. Close releases its lock.
type Owner struct {
	store *Store
	lock  *os.File
	hook  func(string) error // injected only by interruption tests
}

func (s *Store) Acquire() (*Owner, error) {
	if err := ensureDir(s.dir); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(s.dir, "execution.lock"), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrOwned
		}
		return nil, fmt.Errorf("execution lock: %w", err)
	}
	owner := &Owner{store: s, lock: lock}
	if err := owner.recover(); err != nil {
		owner.Close()
		return nil, err
	}
	return owner, nil
}

func (o *Owner) Close() error {
	if o.lock == nil {
		return nil
	}
	err := syscall.Flock(int(o.lock.Fd()), syscall.LOCK_UN)
	closeErr := o.lock.Close()
	o.lock = nil
	if err != nil {
		return err
	}
	return closeErr
}

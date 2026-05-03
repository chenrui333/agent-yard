package task

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenrui333/agent-yard/internal/lock"
	"gopkg.in/yaml.v3"
)

const DefaultFile = "tasks.yaml"

type Store struct {
	Path string
}

func NewStore(path string) Store {
	return Store{Path: path}
}

func (s Store) Load() (Ledger, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EmptyLedger(), nil
		}
		return Ledger{}, fmt.Errorf("load tasks %s: %w", s.Path, err)
	}
	var ledger Ledger
	if err := yaml.Unmarshal(data, &ledger); err != nil {
		return Ledger{}, fmt.Errorf("parse tasks %s: %w", s.Path, err)
	}
	Normalize(&ledger)
	if err := Validate(ledger); err != nil {
		return Ledger{}, err
	}
	return ledger, nil
}

func (s Store) Save(ledger Ledger) error {
	Normalize(&ledger)
	if err := Validate(ledger); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create tasks dir: %w", err)
	}
	lockFile, err := lock.Acquire(s.Path + ".lock")
	if err != nil {
		return err
	}
	defer lockFile.Release()

	data, err := yaml.Marshal(&ledger)
	if err != nil {
		return fmt.Errorf("marshal tasks: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), filepath.Base(s.Path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp tasks file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp tasks file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp tasks file: %w", err)
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return fmt.Errorf("replace tasks %s: %w", s.Path, err)
	}
	return nil
}

func (s Store) Update(id string, update func(*Task) error) error {
	ledger, err := s.Load()
	if err != nil {
		return err
	}
	if err := ledger.Update(id, update); err != nil {
		return err
	}
	return s.Save(ledger)
}

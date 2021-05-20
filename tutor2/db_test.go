package tutor2

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	errShouldFail = errors.New("mock should fail")
	errInvalid    = errors.New("invalid operation")
)

type mockDB struct {
	data map[string]*Item
	fail bool
}

func (m *mockDB) AddItem(_ context.Context, i *Item) (string, error) {
	if m.fail {
		return "", errShouldFail
	}

	if m.data == nil {
		m.data = make(map[string]*Item)
	} else if i.ID != "" {
		return "", errInvalid
	}

add:
	i.ID = uuid.New().String()

	if _, ok := m.data[i.ID]; ok {
		goto add
	}

	m.data[i.ID] = i

	return i.ID, nil
}

func (m *mockDB) GetItem(_ context.Context, id string) (*Item, error) {
	if m.fail {
		return nil, errShouldFail
	}

	if i, ok := m.data[id]; ok {
		return i, nil
	}

	return nil, ErrNotFound
}

func (m *mockDB) ListItems(_ context.Context) ([]*Item, error) {
	if m.fail {
		return nil, errShouldFail
	}

	result := make([]*Item, 0, len(m.data))

	for _, i := range m.data {
		result = append(result, i)
	}

	return result, nil
}

func (m *mockDB) UpdateItem(_ context.Context, i *Item) error {
	if m.fail {
		return errShouldFail
	}

	if _, ok := m.data[i.ID]; !ok {
		return errInvalid
	}

	m.data[i.ID] = i

	return nil
}

func (m *mockDB) DeleteItem(_ context.Context, id string) error {
	if m.fail {
		return errShouldFail
	}

	if m.data != nil {
		delete(m.data, id)
	}

	return nil
}

func (m *mockDB) preload() {
	if m.data == nil {
		m.data = make(map[string]*Item)
	}

	for i := 1; i < 10; i++ {
		id := uuid.New().String()
		item := Item{ID: id, Name: fmt.Sprintf("item-%d", i)}

		m.data[id] = &item
	}
}

package tutor3

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

var (
	errShouldFail = errors.New("mock should fail")
	errInvalid    = errors.New("invalid operation")
)

// mockDB is not thread-safe; we expect to run
// UTs one at a time or with their own mock
type mockDB struct {
	data map[string]*Item
	next int
	fail bool
}

func (m *mockDB) AddItem(_ context.Context, i *Item) (string, error) {
	if m.fail {
		return "", errShouldFail
	}

	if m.data == nil {
		m.data = make(map[string]*Item)
		m.next = 1000
	} else if i.ID != "" {
		return "", errInvalid
	}

add:
	i.ID = uuid.New().String()

	if _, ok := m.data[i.ID]; ok {
		goto add
	}

	i.SKU = m.next
	m.data[i.ID] = i

	m.next++

	return i.ID, nil
}

func (m *mockDB) GetItem(_ context.Context, id string) (*Item, error) {
	if m.fail {
		return nil, errShouldFail
	}

	if i, ok := m.data[id]; ok {
		return i, nil
	}

	return nil, errNotFound
}

func (m *mockDB) GetItemBySKU(_ context.Context, sku int) (*Item, error) {
	if m.fail {
		return nil, errShouldFail
	}

	for _, v := range m.data {
		if v.SKU == sku {
			return v, nil
		}
	}

	return nil, errNotFound
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

func (m *mockDB) ListSKUs(_ context.Context) (map[string]string, error) {
	if m.fail {
		return nil, errShouldFail
	}

	result := make(map[string]string, len(m.data))

	for _, i := range m.data {
		result[strconv.Itoa(i.SKU)] = i.ID
	}

	return result, nil
}

func (m *mockDB) UpdateItem(_ context.Context, i *Item) error {
	if m.fail {
		return errShouldFail
	}

	if m.data == nil {
		return errInvalid
	}

	m.data[i.ID] = i

	return nil
}

func (m *mockDB) DeleteItem(_ context.Context, id string) error {
	if m.fail {
		return errShouldFail
	}

	if m.data == nil {
		return errInvalid
	}

	delete(m.data, id)

	return nil
}

func (m *mockDB) preload() {
	if m.data == nil {
		m.data = make(map[string]*Item)
		m.next = 1000
	}

	for i := 1; i < 10; i++ {
		id := uuid.New().String()
		item := Item{ID: id, Name: fmt.Sprintf("item-%d", i), SKU: m.next}

		m.data[id] = &item
		m.next++
	}
}

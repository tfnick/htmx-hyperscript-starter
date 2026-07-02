package namelookup

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

var ErrLoaderNotRegistered = errors.New("name lookup loader not registered")

type Key string

type Loader func(context.Context, []string) (map[string]string, error)

type Row struct {
	ID   string `db:"id"`
	Name string `db:"name"`
}

type Registration struct {
	key    Key
	loader Loader
}

func Resource(key Key, loader Loader) Registration {
	return Registration{
		key:    key,
		loader: loader,
	}
}

type Registry struct {
	loaders map[Key]Loader
}

func NewRegistry(resources ...Registration) Registry {
	loaders := make(map[Key]Loader, len(resources))
	for _, resource := range resources {
		loaders[resource.key] = resource.loader
	}
	return Registry{loaders: loaders}
}

func (r Registry) Resolve(ctx context.Context, collect func(*Batch)) (Result, error) {
	batch := NewBatch()
	if collect != nil {
		collect(batch)
	}
	return r.ResolveBatch(ctx, batch)
}

func (r Registry) ResolveBatch(ctx context.Context, batch *Batch) (Result, error) {
	result := Result{names: map[Key]map[string]string{}}
	if batch == nil {
		return result, nil
	}

	for _, key := range batch.Keys() {
		ids := batch.IDs(key)
		if len(ids) == 0 {
			continue
		}

		loader := r.loaders[key]
		if loader == nil {
			return Result{}, fmt.Errorf("%w: %s", ErrLoaderNotRegistered, key)
		}

		names, err := loader(ctx, ids)
		if err != nil {
			return Result{}, fmt.Errorf("load names for %s failed: %w", key, err)
		}
		if names == nil {
			names = map[string]string{}
		}
		result.names[key] = copyStringMap(names)
	}
	return result, nil
}

type Batch struct {
	ids  map[Key][]string
	seen map[Key]map[string]struct{}
}

func NewBatch() *Batch {
	return &Batch{
		ids:  map[Key][]string{},
		seen: map[Key]map[string]struct{}{},
	}
}

func (b *Batch) Add(key Key, id string) {
	if b == nil || id == "" {
		return
	}
	if b.seen[key] == nil {
		b.seen[key] = map[string]struct{}{}
	}
	if _, exists := b.seen[key][id]; exists {
		return
	}
	b.seen[key][id] = struct{}{}
	b.ids[key] = append(b.ids[key], id)
}

func (b *Batch) AddMany(key Key, ids []string) {
	for _, id := range ids {
		b.Add(key, id)
	}
}

func (b *Batch) IDs(key Key) []string {
	if b == nil {
		return nil
	}
	return append([]string(nil), b.ids[key]...)
}

func (b *Batch) Keys() []Key {
	if b == nil {
		return nil
	}
	keys := make([]Key, 0, len(b.ids))
	for key := range b.ids {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

type Result struct {
	names map[Key]map[string]string
}

func (r Result) Name(key Key, id string) string {
	if id == "" || r.names == nil || r.names[key] == nil {
		return ""
	}
	return r.names[key][id]
}

func (r Result) Map(key Key) map[string]string {
	if r.names == nil || r.names[key] == nil {
		return map[string]string{}
	}
	return copyStringMap(r.names[key])
}

func UniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func RowsToMap(rows []Row) map[string]string {
	names := make(map[string]string, len(rows))
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names
}

func Load(ctx context.Context, ids []string, loader Loader) (map[string]string, error) {
	uniqueIDs := UniqueNonEmpty(ids)
	if len(uniqueIDs) == 0 {
		return map[string]string{}, nil
	}
	return loader(ctx, uniqueIDs)
}

func Collect[T any](batch *Batch, key Key, items []T, id func(T) string) {
	if batch == nil || id == nil {
		return
	}
	for _, item := range items {
		batch.Add(key, id(item))
	}
}

func copyStringMap(values map[string]string) map[string]string {
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

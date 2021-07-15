package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/filecoin-project/sentinel-visor/config"
	"github.com/filecoin-project/sentinel-visor/model"
	"golang.org/x/xerrors"
)

type Connector interface {
	Connect(context.Context) error
	IsConnected(context.Context) bool
	Close(context.Context) error
}

func NewCatalog(cfg config.StorageConf) (*Catalog, error) {
	c := &Catalog{
		storages: make(map[string]model.Storage),
	}

	for name, sc := range cfg.Postgresql {
		if _, exists := c.storages[name]; exists {
			return nil, fmt.Errorf("duplicate storage name: %q", name)
		}
		log.Debugw("registering storage", "name", name, "type", "postgresql")

		// Find the url of the database, which is either indirectly specified using URLEnv or explicit via URL
		var dburl string
		if sc.URLEnv != "" {
			dburl = os.Getenv(sc.URLEnv)
		} else {
			dburl = sc.URL
		}

		db, err := NewDatabase(context.TODO(), dburl, sc.PoolSize, sc.ApplicationName, sc.SchemaName, sc.AllowUpsert)
		if err != nil {
			return nil, fmt.Errorf("failed to create postgresql storage %q: %w", name, err)
		}

		c.storages[name] = db
	}

	for name, sc := range cfg.File {
		if _, exists := c.storages[name]; exists {
			return nil, fmt.Errorf("duplicate storage name: %q", name)
		}

		switch sc.Format {
		case "CSV":
			log.Debugw("registering storage", "name", name, "type", "csv")

			db, err := NewCSVStorageLatest(sc.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to create postgresql storage %q: %w", name, err)
			}
			c.storages[name] = db

		default:
			return nil, fmt.Errorf("unsupported format %q for storage %q", sc.Format, name)
		}

	}

	return c, nil
}

// A Catalog holds a list of pre-configured storage systems and can open them when requested.
type Catalog struct {
	storages map[string]model.Storage
}

// Connect returns a storage that is ready for use. If name is empty, a null storage will be returned
func (c *Catalog) Connect(ctx context.Context, name string) (model.Storage, error) {
	if name == "" {
		return &NullStorage{}, nil
	}

	s, exists := c.storages[name]
	if !exists {
		return nil, fmt.Errorf("unknown storage: %q", name)
	}

	// Does this storage need to be connected?
	cs, ok := s.(Connector)
	if ok {
		if !cs.IsConnected(ctx) {
			err := cs.Connect(ctx)
			if err != nil {
				return nil, err
			}
		}
	}

	return s, nil
}

// ConnectAsDatabase returns a storage that is ready to use for reading and writing: `name` must corresponds to
// a Database storage.
func (c *Catalog) ConnectAsDatabase(ctx context.Context, name string) (*Database, error) {
	strg, err := c.Connect(ctx, name)
	if err != nil {
		return nil, err
	}

	db, ok := strg.(*Database)
	if !ok {
		return nil, xerrors.Errorf("connecting to storage as database. unexpected type: %T", strg)
	}
	return db, nil
}

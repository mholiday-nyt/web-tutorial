package tutor2

import (
	"context"
	"errors"
	"fmt"
	"log"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DB interface {
	AddItem(context.Context, *Item) (string, error)
	GetItem(context.Context, string) (*Item, error)
	ListItems(context.Context) ([]*Item, error)
	UpdateItem(context.Context, *Item) error
	DeleteItem(context.Context, string) error
}

type Client struct {
	fs   *firestore.Client
	data *firestore.CollectionRef
}

func NewClient(project, collection string) (*Client, error) {
	if project == "" {
		return nil, errors.New("no projectID")
	}

	client, err := firestore.NewClient(context.Background(), project)

	if err != nil {
		return nil, fmt.Errorf("failed to create FS client: %w", err)
	}

	c := Client{
		fs:   client,
		data: client.Collection(collection),
	}

	return &c, nil
}

func (c *Client) Close() {
	c.fs.Close()
}

type Item struct {
	ID   string `json:"id" firestore:"id"`
	Name string `json:"name" firestore:"name"`
}

var ErrNotFound = errors.New("not found")

func (c *Client) AddItem(ctx context.Context, i *Item) (string, error) {
	var ref *firestore.DocumentRef

add:
	i.ID = uuid.New().String()
	ref = c.data.Doc(i.ID)

	if _, err := ref.Create(ctx, i); err != nil {
		// it's unlikely to happen even once and virtually
		// impossible for it to happen twice in a row

		if status.Code(err) == codes.AlreadyExists {
			goto add
		}

		return "", err
	}

	return i.ID, nil
}

func (c *Client) GetItem(ctx context.Context, id string) (*Item, error) {
	doc, err := c.data.Doc(id).Get(ctx)

	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("%s: %w", id, ErrNotFound)
		}

		return nil, fmt.Errorf("item %s: %w", id, err)
	}

	var i Item

	if err = doc.DataTo(&i); err != nil {
		return nil, fmt.Errorf("item %s decode: %w", id, err)
	}

	return &i, nil
}

func (c *Client) ListItems(ctx context.Context) ([]*Item, error) {
	query := c.data.OrderBy(firestore.DocumentID, firestore.Asc)
	docs, err := query.Documents(ctx).GetAll()

	if err != nil {
		return nil, err
	}

	result := make([]*Item, 0, len(docs))

	for _, doc := range docs {
		var i Item

		if err = doc.DataTo(&i); err != nil {
			log.Printf("item %s decode: %s", doc.Ref.ID, err)
			continue
		}

		result = append(result, &i)
	}

	return result, nil
}

func (c *Client) UpdateItem(ctx context.Context, i *Item) error {
	ref := c.data.Doc(i.ID)

	// set can create or overwrite existing data
	// so we need to see if it exists first

	if _, err := ref.Get(ctx); err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("%s: %w", i.ID, ErrNotFound)
		}

		return err
	}

	if _, err := ref.Set(ctx, i); err != nil {
		return err
	}

	return nil
}

func (c *Client) DeleteItem(ctx context.Context, id string) error {
	_, err := c.data.Doc(id).Delete(ctx)

	if err != nil {
		return err
	}

	return nil
}
